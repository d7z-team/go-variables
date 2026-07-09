package variables

import (
	"fmt"
	"reflect"
)

type StaticKind uint8

const (
	TypeUnknown StaticKind = iota
	TypeAny
	TypeNull
	TypeBool
	TypeString
	TypeNumber
	TypeInteger
	TypeFloat
	TypeObject
	TypeArray
)

type StaticType struct {
	Kind     StaticKind
	Nullable bool
	Elem     *StaticType
	Fields   map[string]StaticType
	Elements []StaticType
	Selected bool
	Const    *Value
}

type FunctionSpec struct {
	Name       string
	MinArgs    int
	MaxArgs    int
	Params     []StaticType
	Variadic   *StaticType
	ReturnType StaticType
	Return     func([]StaticType) (StaticType, error)
	Runtime    Function
}

type CompileOption func(*compileOptions)
type ExpressionOption = CompileOption

type compileOptions struct {
	root   StaticType
	funcs  map[string]FunctionSpec
	strict bool
}

func WithRootValue(value Value) CompileOption {
	return func(opts *compileOptions) {
		opts.root = InferStaticType(value)
	}
}

func WithRootSchema(schema StaticType) CompileOption {
	return func(opts *compileOptions) {
		opts.root = schema
	}
}

func WithFunctionSpecs(specs map[string]FunctionSpec) CompileOption {
	return func(opts *compileOptions) {
		if opts.funcs == nil {
			opts.funcs = map[string]FunctionSpec{}
		}
		for name, spec := range specs {
			spec.Name = name
			opts.funcs[name] = spec
		}
	}
}

func WithExpressionFunctions(funcs map[string]any) ExpressionOption {
	return func(opts *compileOptions) {
		if opts.funcs == nil {
			opts.funcs = map[string]FunctionSpec{}
		}
		for name, fn := range funcs {
			opts.funcs[name] = goFunctionSpec(name, fn)
		}
	}
}

func WithStrictTypes() CompileOption {
	return func(opts *compileOptions) {
		opts.strict = true
	}
}

func InferStaticType(value Value) StaticType {
	out := StaticType{Kind: kindToStatic(value.kind)}
	switch value.kind {
	case NullValue:
		out.Nullable = true
	case NumberValue:
		if value.n.kind == IntegerNumber {
			out.Kind = TypeInteger
		} else {
			out.Kind = TypeFloat
		}
	case ObjectValue:
		out.Fields = make(map[string]StaticType, len(value.obj))
		for key, child := range value.obj {
			out.Fields[key] = InferStaticType(child)
		}
	case ArrayValue:
		out.Elements = make([]StaticType, len(value.arr))
		var elem StaticType
		for i, child := range value.arr {
			childType := InferStaticType(child)
			out.Elements[i] = childType
			elem = mergeStaticTypes(elem, childType)
		}
		out.Elem = &elem
	}
	return out
}

func unknownFunctionSpec(name string, fn Function) FunctionSpec {
	return FunctionSpec{Name: name, MinArgs: 0, MaxArgs: -1, ReturnType: unknownType(), Runtime: fn}
}

func goFunctionSpec(name string, fn any) FunctionSpec {
	spec := FunctionSpec{Name: name, MinArgs: 0, MaxArgs: -1, ReturnType: unknownType(), Runtime: AdaptFunction(fn)}
	value := reflect.ValueOf(fn)
	if !value.IsValid() || value.Kind() != reflect.Func {
		spec.Return = func([]StaticType) (StaticType, error) {
			return StaticType{}, fmt.Errorf("custom function %q must be func, got %T", name, fn)
		}
		return spec
	}
	fnType := value.Type()
	argStart := 0
	if fnType.NumIn() > 0 && fnType.In(0) == reflect.TypeOf(FunctionContext{}) {
		argStart = 1
	}
	spec.MinArgs = fnType.NumIn() - argStart
	spec.MaxArgs = spec.MinArgs
	if fnType.IsVariadic() {
		spec.MinArgs--
		spec.MaxArgs = -1
		variadic := reflectStaticType(fnType.In(fnType.NumIn() - 1).Elem())
		spec.Variadic = &variadic
	}
	for i := argStart; i < fnType.NumIn(); i++ {
		param := reflectStaticType(fnType.In(i))
		if fnType.IsVariadic() && i == fnType.NumIn()-1 {
			param = reflectStaticType(fnType.In(i).Elem())
		}
		spec.Params = append(spec.Params, param)
	}
	switch fnType.NumOut() {
	case 0:
		spec.ReturnType = nullType()
	case 1:
		if fnType.Out(0).Implements(errorType) {
			spec.ReturnType = nullType()
		} else {
			spec.ReturnType = reflectStaticType(fnType.Out(0))
		}
	case 2:
		if !fnType.Out(1).Implements(errorType) {
			spec.Return = func([]StaticType) (StaticType, error) {
				return StaticType{}, fmt.Errorf("custom function %q second return must be error", name)
			}
		}
		spec.ReturnType = reflectStaticType(fnType.Out(0))
	default:
		spec.Return = func([]StaticType) (StaticType, error) {
			return StaticType{}, fmt.Errorf("custom function %q returns too many values", name)
		}
	}
	return spec
}

func checkExpression(expr Expression, cfg compileOptions) error {
	ctx := checkContext{root: cfg.root, current: cfg.root, funcs: cfg.funcs, strict: cfg.strict}
	if ctx.root.Kind == 0 {
		ctx.root = unknownType()
		ctx.current = ctx.root
	}
	_, err := inferExpr(expr.root, &ctx)
	return err
}

type checkContext struct {
	root       StaticType
	current    StaticType
	funcs      map[string]FunctionSpec
	strict     bool
	filterMode bool
}

func inferExpr(node exprNode, ctx *checkContext) (StaticType, error) {
	switch n := node.(type) {
	case literalNode:
		value, err := EncodeValue(n.value)
		if err != nil {
			return StaticType{}, err
		}
		t := InferStaticType(value)
		t.Const = &value
		return t, nil
	case arrayNode:
		out := StaticType{Kind: TypeArray, Elements: make([]StaticType, len(n.items))}
		values := make([]Value, len(n.items))
		constant := true
		for i, item := range n.items {
			itemType, err := inferExpr(item, ctx)
			if err != nil {
				return StaticType{}, err
			}
			out.Elements[i] = itemType
			elem := derefType(out.Elem)
			elem = mergeStaticTypes(elem, itemType)
			out.Elem = &elem
			if itemType.Const == nil {
				constant = false
				continue
			}
			values[i] = itemType.Const.Clone()
		}
		if out.Elem == nil {
			elem := unknownType()
			out.Elem = &elem
		}
		if constant {
			value := Array(values)
			out.Const = &value
		}
		return out, nil
	case objectNode:
		out := StaticType{Kind: TypeObject, Fields: map[string]StaticType{}}
		values := map[string]Value{}
		constant := true
		for _, key := range n.keys {
			valueType, err := inferExpr(n.items[key], ctx)
			if err != nil {
				return StaticType{}, err
			}
			out.Fields[key] = valueType
			if valueType.Const == nil {
				constant = false
				continue
			}
			values[key] = valueType.Const.Clone()
		}
		if constant {
			value := Object(values)
			out.Const = &value
		}
		return out, nil
	case identifierNode:
		if field, ok := staticField(ctx.current, n.name); ok {
			field.Selected = true
			return field, nil
		}
		if ctx.filterMode {
			if !isKnownObject(ctx.current) {
				return unknownType(), nil
			}
			return nullableType(nullType()), nil
		}
		if ctx.strict || isKnownObject(ctx.current) {
			return StaticType{}, typeError(n.pos, "unknown variable %q", n.name)
		}
		return unknownSelectedType(), nil
	case rootNode:
		out := ctx.root
		out.Selected = true
		return out, nil
	case memberNode:
		receiver, err := inferExpr(n.receiver, ctx)
		if err != nil {
			return StaticType{}, err
		}
		return inferMemberType(receiver, n.name, n.optional || ctx.filterMode, n.pos)
	case indexNode:
		receiver, err := inferExpr(n.receiver, ctx)
		if err != nil {
			return StaticType{}, err
		}
		index, err := inferExpr(n.index, ctx)
		if err != nil {
			return StaticType{}, err
		}
		return inferIndexType(receiver, index, n.pos)
	case filterNode:
		receiver, err := inferExpr(n.receiver, ctx)
		if err != nil {
			return StaticType{}, err
		}
		candidate := arrayElemType(receiver)
		child := *ctx
		child.current = candidate
		child.filterMode = true
		if _, err := inferExpr(n.predicate, &child); err != nil {
			return StaticType{}, err
		}
		out := receiver
		out.Kind = TypeArray
		out.Selected = true
		return out, nil
	case unaryNode:
		child, err := inferExpr(n.child, ctx)
		if err != nil {
			return StaticType{}, err
		}
		if n.op == tokenMinus && isKnown(child) && !isNumberLike(child) {
			return StaticType{}, typeError(n.pos, "unary minus expects number, got %s", staticKindName(child.Kind))
		}
		if n.op == tokenMinus {
			out := stripConst(child)
			if child.Const != nil && child.Const.kind == NumberValue {
				value := Value{kind: NumberValue, n: negNumber(child.Const.n)}
				out.Const = &value
			}
			return out, nil
		}
		return boolType(), nil
	case binaryNode:
		return inferBinaryType(n, ctx)
	case callNode:
		args, err := inferArgs(n.args, ctx)
		if err != nil {
			return StaticType{}, err
		}
		return inferCallType(n.name, args, ctx, n.pos)
	case methodCallNode:
		receiver, err := inferExpr(n.receiver, ctx)
		if err != nil {
			return StaticType{}, err
		}
		args, err := inferArgs(n.args, ctx)
		if err != nil {
			return StaticType{}, err
		}
		args = append([]StaticType{receiver}, args...)
		return inferCallType(n.name, args, ctx, n.pos)
	default:
		return unknownType(), nil
	}
}

func inferArgs(nodes []exprNode, ctx *checkContext) ([]StaticType, error) {
	args := make([]StaticType, len(nodes))
	for i, node := range nodes {
		arg, err := inferExpr(node, ctx)
		if err != nil {
			return nil, err
		}
		args[i] = arg
	}
	return args, nil
}

func inferBinaryType(n binaryNode, ctx *checkContext) (StaticType, error) {
	left, err := inferExpr(n.left, ctx)
	if err != nil {
		return StaticType{}, err
	}
	right, err := inferExpr(n.right, ctx)
	if err != nil {
		return StaticType{}, err
	}
	switch n.op {
	case tokenAnd, tokenOr, tokenEqual, tokenNotEqual:
		return boolType(), nil
	case tokenGreater, tokenGreaterEqual, tokenLess, tokenLessEqual:
		if isKnown(left) && isKnown(right) && !typesComparable(left, right) {
			return StaticType{}, typeError(n.pos, "cannot compare %s and %s", staticKindName(left.Kind), staticKindName(right.Kind))
		}
		return boolType(), nil
	case tokenIn:
		if isKnown(right) && right.Kind != TypeArray && right.Kind != TypeString {
			return StaticType{}, typeError(n.pos, "in expects array or string, got %s", staticKindName(right.Kind))
		}
		if right.Kind == TypeString && isKnown(left) && left.Kind != TypeString {
			return StaticType{}, typeError(n.pos, "string contains expects string needle")
		}
		return boolType(), nil
	case tokenPlus:
		if left.Kind == TypeString || right.Kind == TypeString {
			if isKnown(left) && left.Kind != TypeString || isKnown(right) && right.Kind != TypeString {
				return StaticType{}, typeError(n.pos, "cannot add %s and %s", staticKindName(left.Kind), staticKindName(right.Kind))
			}
			return stringType(), nil
		}
		return inferNumericBinary(n.op, left, right, n.pos)
	case tokenMinus, tokenStar, tokenSlash, tokenPercent:
		return inferNumericBinary(n.op, left, right, n.pos)
	default:
		return unknownType(), nil
	}
}

func inferNumericBinary(op tokenKind, left StaticType, right StaticType, pos int) (StaticType, error) {
	if isKnown(left) && !isNumberLike(left) || isKnown(right) && !isNumberLike(right) {
		return StaticType{}, typeError(pos, "arithmetic expects numbers, got %s and %s", staticKindName(left.Kind), staticKindName(right.Kind))
	}
	if op == tokenPercent && ((isKnown(left) && left.Kind != TypeInteger) || (isKnown(right) && right.Kind != TypeInteger)) {
		return StaticType{}, typeError(pos, "modulo expects integers")
	}
	if (op == tokenSlash || op == tokenPercent) && right.Const != nil && right.Const.kind == NumberValue && right.Const.n.isZero() {
		if op == tokenSlash {
			return StaticType{}, typeError(pos, "division by zero")
		}
		return StaticType{}, typeError(pos, "modulo by zero")
	}
	if op == tokenSlash {
		return numberType(), nil
	}
	if left.Kind == TypeInteger && right.Kind == TypeInteger {
		return integerType(), nil
	}
	return numberType(), nil
}

func inferCallType(name string, args []StaticType, ctx *checkContext, pos int) (StaticType, error) {
	spec, ok := builtinFunctionSpecs[name]
	if !ok && ctx.funcs != nil {
		spec, ok = ctx.funcs[name]
	}
	if !ok {
		return StaticType{}, typeError(pos, "unknown function %q", name)
	}
	out, err := spec.check(args)
	if err != nil {
		return StaticType{}, typeError(pos, "%v", err)
	}
	return out, nil
}

func (spec FunctionSpec) check(args []StaticType) (StaticType, error) {
	if spec.MaxArgs >= 0 && len(args) > spec.MaxArgs || len(args) < spec.MinArgs {
		if spec.MaxArgs < 0 {
			return StaticType{}, fmt.Errorf("%s expects at least %d arguments, got %d", spec.Name, spec.MinArgs, len(args))
		}
		if spec.MinArgs == spec.MaxArgs {
			return StaticType{}, fmt.Errorf("%s expects %d arguments, got %d", spec.Name, spec.MinArgs, len(args))
		}
		return StaticType{}, fmt.Errorf("%s expects between %d and %d arguments, got %d", spec.Name, spec.MinArgs, spec.MaxArgs, len(args))
	}
	for i, arg := range args {
		var want StaticType
		if i < len(spec.Params) {
			want = spec.Params[i]
		} else if spec.Variadic != nil {
			want = *spec.Variadic
		} else {
			continue
		}
		if !staticAssignable(arg, want) {
			return StaticType{}, fmt.Errorf("%s argument %d expects %s, got %s", spec.Name, i+1, staticKindName(want.Kind), staticKindName(arg.Kind))
		}
	}
	if spec.Return != nil {
		return spec.Return(args)
	}
	if spec.ReturnType.Kind == 0 {
		return unknownType(), nil
	}
	return spec.ReturnType, nil
}

func inferMemberType(receiver StaticType, name string, optional bool, pos int) (StaticType, error) {
	if receiver.Kind == TypeArray {
		if len(receiver.Elements) > 0 {
			fieldType := unknownType()
			for _, elem := range receiver.Elements {
				field, ok := staticField(elem, name)
				if !ok {
					if optional || !isKnownObject(elem) {
						fieldType = mergeStaticTypes(fieldType, nullableType(unknownType()))
						continue
					}
					return StaticType{}, typeError(pos, "missing field %q", name)
				}
				fieldType = mergeStaticTypes(fieldType, field)
			}
			return arrayOf(fieldType), nil
		}
		elem := arrayElemType(receiver)
		field, ok := staticField(elem, name)
		if !ok {
			if optional || !isKnownObject(elem) {
				return arrayOf(nullableType(unknownType())), nil
			}
			return StaticType{}, typeError(pos, "missing field %q", name)
		}
		return arrayOf(field), nil
	}
	field, ok := staticField(receiver, name)
	if !ok {
		if optional || !isKnownObject(receiver) {
			return nullableType(unknownType()), nil
		}
		return StaticType{}, typeError(pos, "missing field %q", name)
	}
	field.Selected = true
	return field, nil
}

func inferIndexType(receiver StaticType, index StaticType, pos int) (StaticType, error) {
	switch receiver.Kind {
	case TypeArray:
		if isKnown(index) && index.Kind != TypeInteger {
			return StaticType{}, typeError(pos, "array index must be integer")
		}
		if index.Const != nil && len(receiver.Elements) > 0 {
			i, ok := valueIndex(*index.Const)
			if ok {
				i = normalizeIndex(i, len(receiver.Elements))
				if i < 0 || i >= len(receiver.Elements) {
					return StaticType{}, typeError(pos, "array index out of range")
				}
				return receiver.Elements[i], nil
			}
		}
		return arrayElemType(receiver), nil
	case TypeObject:
		if isKnown(index) && index.Kind != TypeString {
			return StaticType{}, typeError(pos, "object index must be string")
		}
		if index.Const != nil {
			key, ok := index.Const.StringValue()
			if ok {
				if field, ok := staticField(receiver, key); ok {
					return field, nil
				}
				return StaticType{}, typeError(pos, "missing field %q", key)
			}
		}
		return unknownType(), nil
	default:
		if isKnown(receiver) {
			return StaticType{}, typeError(pos, "index expects array or object, got %s", staticKindName(receiver.Kind))
		}
		return unknownType(), nil
	}
}

func staticField(value StaticType, name string) (StaticType, bool) {
	if value.Kind != TypeObject || value.Fields == nil {
		return StaticType{}, false
	}
	field, ok := value.Fields[name]
	return field, ok
}

func builtinArity(name string, runtime Function, min int, max int, ret func([]StaticType) (StaticType, error)) FunctionSpec {
	return FunctionSpec{Name: name, MinArgs: min, MaxArgs: max, Runtime: runtime, Return: ret}
}

func reflectStaticType(t reflect.Type) StaticType {
	if t == reflect.TypeOf(Value{}) {
		return unknownType()
	}
	if t.Kind() == reflect.Interface {
		return anyType()
	}
	switch t.Kind() {
	case reflect.Bool:
		return boolType()
	case reflect.String:
		return stringType()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return integerType()
	case reflect.Float32, reflect.Float64:
		return floatType()
	case reflect.Slice, reflect.Array:
		elem := reflectStaticType(t.Elem())
		return arrayOf(elem)
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return unknownType()
		}
		elem := reflectStaticType(t.Elem())
		return StaticType{Kind: TypeObject, Elem: &elem}
	default:
		return unknownType()
	}
}

func kindToStatic(kind ValueKind) StaticKind {
	switch kind {
	case NullValue:
		return TypeNull
	case BoolValue:
		return TypeBool
	case StringValue:
		return TypeString
	case NumberValue:
		return TypeNumber
	case ObjectValue:
		return TypeObject
	case ArrayValue:
		return TypeArray
	default:
		return TypeUnknown
	}
}

func mergeStaticTypes(left StaticType, right StaticType) StaticType {
	if left.Kind == 0 || left.Kind == TypeUnknown {
		return stripConst(right)
	}
	if right.Kind == 0 || right.Kind == TypeUnknown {
		return stripConst(left)
	}
	if left.Kind == right.Kind {
		out := stripConst(left)
		out.Nullable = left.Nullable || right.Nullable
		if left.Kind == TypeArray {
			elem := mergeStaticTypes(derefType(left.Elem), derefType(right.Elem))
			out.Elem = &elem
		} else if left.Kind == TypeObject && (left.Fields != nil || right.Fields != nil) {
			out.Fields = map[string]StaticType{}
			for key, field := range left.Fields {
				if rightField, ok := right.Fields[key]; ok {
					out.Fields[key] = mergeStaticTypes(field, rightField)
				} else {
					out.Fields[key] = nullableType(stripConst(field))
				}
			}
			for key, field := range right.Fields {
				if _, ok := out.Fields[key]; !ok {
					out.Fields[key] = nullableType(stripConst(field))
				}
			}
		}
		return out
	}
	if isNumberLike(left) && isNumberLike(right) {
		return numberType()
	}
	if left.Kind == TypeNull {
		out := stripConst(right)
		out.Nullable = true
		return out
	}
	if right.Kind == TypeNull {
		out := stripConst(left)
		out.Nullable = true
		return out
	}
	return anyType()
}

func arrayElemType(value StaticType) StaticType {
	if value.Elem != nil {
		return *value.Elem
	}
	return unknownType()
}

func derefType(value *StaticType) StaticType {
	if value == nil {
		return unknownType()
	}
	return *value
}

func staticAssignable(got StaticType, want StaticType) bool {
	if want.Kind == TypeAny || want.Kind == TypeUnknown || got.Kind == TypeUnknown || got.Kind == TypeAny {
		return true
	}
	if got.Kind == TypeNull {
		return want.Nullable || want.Kind == TypeNull
	}
	if want.Kind == TypeNumber {
		return isNumberLike(got)
	}
	if got.Kind != want.Kind {
		return false
	}
	switch want.Kind {
	case TypeArray:
		if want.Elem == nil {
			return true
		}
		for _, item := range got.Elements {
			if !staticAssignable(item, *want.Elem) {
				return false
			}
		}
		if got.Elem != nil {
			return staticAssignable(*got.Elem, *want.Elem)
		}
	case TypeObject:
		if want.Elem != nil {
			for _, field := range got.Fields {
				if !staticAssignable(field, *want.Elem) {
					return false
				}
			}
			if got.Elem != nil {
				return staticAssignable(*got.Elem, *want.Elem)
			}
		}
		for key, wantField := range want.Fields {
			gotField, ok := got.Fields[key]
			if !ok || !staticAssignable(gotField, wantField) {
				return false
			}
		}
	}
	return true
}

func typesComparable(left StaticType, right StaticType) bool {
	return isNumberLike(left) && isNumberLike(right) || left.Kind == TypeString && right.Kind == TypeString
}

func isKnown(value StaticType) bool {
	return value.Kind != TypeUnknown && value.Kind != TypeAny && value.Kind != 0
}

func isKnownObject(value StaticType) bool {
	return value.Kind == TypeObject && value.Fields != nil
}

func isNumberLike(value StaticType) bool {
	return value.Kind == TypeNumber || value.Kind == TypeInteger || value.Kind == TypeFloat
}

func stripConst(value StaticType) StaticType {
	value.Const = nil
	return value
}

func nullableType(value StaticType) StaticType {
	value.Nullable = true
	return value
}

func unknownType() StaticType { return StaticType{Kind: TypeUnknown} }
func unknownSelectedType() StaticType {
	value := unknownType()
	value.Selected = true
	return value
}
func anyType() StaticType     { return StaticType{Kind: TypeAny} }
func nullType() StaticType    { return StaticType{Kind: TypeNull, Nullable: true} }
func boolType() StaticType    { return StaticType{Kind: TypeBool} }
func stringType() StaticType  { return StaticType{Kind: TypeString} }
func numberType() StaticType  { return StaticType{Kind: TypeNumber} }
func integerType() StaticType { return StaticType{Kind: TypeInteger} }
func floatType() StaticType   { return StaticType{Kind: TypeFloat} }
func arrayOf(elem StaticType) StaticType {
	return StaticType{Kind: TypeArray, Elem: &elem}
}

func typeError(pos int, format string, args ...any) error {
	if pos > 0 {
		return fmt.Errorf("expression type error at byte %d: %s", pos, fmt.Sprintf(format, args...))
	}
	return fmt.Errorf("expression type error: %s", fmt.Sprintf(format, args...))
}

func staticKindName(kind StaticKind) string {
	switch kind {
	case TypeUnknown:
		return "unknown"
	case TypeAny:
		return "any"
	case TypeNull:
		return "null"
	case TypeBool:
		return "bool"
	case TypeString:
		return "string"
	case TypeNumber:
		return "number"
	case TypeInteger:
		return "integer"
	case TypeFloat:
		return "float"
	case TypeObject:
		return "object"
	case TypeArray:
		return "array"
	default:
		return "unknown"
	}
}
