package variables

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

var builtinFunctionSpecs = map[string]FunctionSpec{
	"len":        builtinArity("len", builtinLen, 1, 1, checkLen),
	"count":      builtinArity("count", builtinLen, 1, 1, checkLen),
	"exists":     builtinArity("exists", builtinExists, 1, 1, func([]StaticType) (StaticType, error) { return boolType(), nil }),
	"contains":   builtinArity("contains", builtinContains, 2, 2, checkContains),
	"first":      builtinArity("first", builtinFirst, 1, 1, checkFirstLast),
	"last":       builtinArity("last", builtinLast, 1, 1, checkFirstLast),
	"sum":        builtinArity("sum", builtinSum, 1, 1, checkSum),
	"avg":        builtinArity("avg", builtinAvg, 1, 1, checkAvg),
	"min":        builtinArity("min", builtinMin, 1, 1, checkMinMax),
	"max":        builtinArity("max", builtinMax, 1, 1, checkMinMax),
	"sort":       builtinArity("sort", builtinSort, 1, 1, checkSort),
	"sortDesc":   builtinArity("sortDesc", builtinSortDesc, 1, 1, checkSort),
	"sortBy":     builtinArity("sortBy", builtinSortBy, 2, 2, checkSortBy),
	"sortByDesc": builtinArity("sortByDesc", builtinSortByDesc, 2, 2, checkSortBy),
	"keys":       builtinArity("keys", builtinKeys, 1, 1, checkKeys),
	"values":     builtinArity("values", builtinValues, 1, 1, checkValues),
	"unique":     builtinArity("unique", builtinUnique, 1, 1, checkArrayPassthrough),
	"compact":    builtinArity("compact", builtinCompact, 1, 1, checkArrayPassthrough),
	"join":       builtinArity("join", builtinJoin, 2, 2, checkJoin),
	"default":    builtinArity("default", builtinDefault, 2, 2, checkDefault),
}

func checkLen(args []StaticType) (StaticType, error) {
	arg := args[0]
	if isKnown(arg) && arg.Kind != TypeNull && arg.Kind != TypeString && arg.Kind != TypeArray && arg.Kind != TypeObject {
		return StaticType{}, fmt.Errorf("len expects string, array, object, or null, got %s", staticKindName(arg.Kind))
	}
	return integerType(), nil
}

func checkContains(args []StaticType) (StaticType, error) {
	container, needle := args[0], args[1]
	if !isKnown(container) {
		return boolType(), nil
	}
	switch container.Kind {
	case TypeString:
		if isKnown(needle) && needle.Kind != TypeString {
			return StaticType{}, fmt.Errorf("string contains expects string needle")
		}
	case TypeArray:
	default:
		return StaticType{}, fmt.Errorf("contains expects array or string, got %s", staticKindName(container.Kind))
	}
	return boolType(), nil
}

func checkFirstLast(args []StaticType) (StaticType, error) {
	elem, err := checkedArrayElem("first", args[0])
	if err != nil {
		return StaticType{}, err
	}
	return nullableType(elem), nil
}

func checkSum(args []StaticType) (StaticType, error) {
	elem, err := checkedArrayElem("sum", args[0])
	if err != nil {
		return StaticType{}, err
	}
	if err := requireNumberElements("sum", args[0], elem); err != nil {
		return StaticType{}, err
	}
	if elem.Kind == TypeInteger {
		return integerType(), nil
	}
	return numberType(), nil
}

func checkAvg(args []StaticType) (StaticType, error) {
	elem, err := checkedArrayElem("avg", args[0])
	if err != nil {
		return StaticType{}, err
	}
	if err := requireNumberElements("avg", args[0], elem); err != nil {
		return StaticType{}, err
	}
	return nullableType(numberType()), nil
}

func checkMinMax(args []StaticType) (StaticType, error) {
	elem, err := checkedArrayElem("min", args[0])
	if err != nil {
		return StaticType{}, err
	}
	if err := requireComparableElements("min", args[0], elem); err != nil {
		return StaticType{}, err
	}
	return nullableType(elem), nil
}

func checkSort(args []StaticType) (StaticType, error) {
	elem, err := checkedArrayElem("sort", args[0])
	if err != nil {
		return StaticType{}, err
	}
	if err := requireComparableElements("sort", args[0], elem); err != nil {
		return StaticType{}, err
	}
	return stripConst(args[0]), nil
}

func checkSortBy(args []StaticType) (StaticType, error) {
	list := args[0]
	elem, err := checkedArrayElem("sortBy", list)
	if err != nil {
		return StaticType{}, err
	}
	if isKnown(args[1]) && args[1].Kind != TypeString {
		return StaticType{}, fmt.Errorf("sortBy field must be string")
	}
	if args[1].Const != nil && isKnownObject(elem) {
		field, _ := args[1].Const.StringValue()
		fieldType, ok := fieldByStaticPath(elem, field)
		if !ok {
			return StaticType{}, fmt.Errorf("missing sort field %q", field)
		}
		if !typeComparableAlone(fieldType) {
			return StaticType{}, fmt.Errorf("sort field %q is not comparable", field)
		}
	}
	return stripConst(list), nil
}

func checkKeys(args []StaticType) (StaticType, error) {
	if isKnown(args[0]) && args[0].Kind != TypeObject {
		return StaticType{}, fmt.Errorf("keys expects object, got %s", staticKindName(args[0].Kind))
	}
	return arrayOf(stringType()), nil
}

func checkValues(args []StaticType) (StaticType, error) {
	if isKnown(args[0]) && args[0].Kind != TypeObject {
		return StaticType{}, fmt.Errorf("values expects object, got %s", staticKindName(args[0].Kind))
	}
	out := unknownType()
	if args[0].Fields != nil {
		for _, field := range args[0].Fields {
			out = mergeStaticTypes(out, field)
		}
	}
	return arrayOf(out), nil
}

func checkArrayPassthrough(args []StaticType) (StaticType, error) {
	if _, err := checkedArrayElem("array function", args[0]); err != nil {
		return StaticType{}, err
	}
	return stripConst(args[0]), nil
}

func checkJoin(args []StaticType) (StaticType, error) {
	if _, err := checkedArrayElem("join", args[0]); err != nil {
		return StaticType{}, err
	}
	if isKnown(args[1]) && args[1].Kind != TypeString {
		return StaticType{}, fmt.Errorf("join separator must be string")
	}
	return stringType(), nil
}

func checkDefault(args []StaticType) (StaticType, error) {
	if args[0].Const != nil && args[0].Const.kind == NullValue {
		return stripConst(args[1]), nil
	}
	if args[0].Const != nil && args[0].Const.kind != NullValue {
		return stripConst(args[0]), nil
	}
	return mergeStaticTypes(args[0], args[1]), nil
}

func checkedArrayElem(name string, value StaticType) (StaticType, error) {
	if value.Kind == TypeNull || !isKnown(value) {
		return unknownType(), nil
	}
	if value.Kind != TypeArray {
		return StaticType{}, fmt.Errorf("%s expects array, got %s", name, staticKindName(value.Kind))
	}
	return arrayElemType(value), nil
}

func requireNumberElements(name string, array StaticType, elem StaticType) error {
	for _, item := range array.Elements {
		if isKnown(item) && !isNumberLike(item) {
			return fmt.Errorf("%s expects numbers, got %s", name, staticKindName(item.Kind))
		}
	}
	if isKnown(elem) && !isNumberLike(elem) && elem.Kind != TypeNull {
		return fmt.Errorf("%s expects numbers, got %s", name, staticKindName(elem.Kind))
	}
	return nil
}

func requireComparableElements(name string, array StaticType, elem StaticType) error {
	for _, item := range array.Elements {
		if isKnown(item) && !typeComparableAlone(item) {
			return fmt.Errorf("%s expects comparable values, got %s", name, staticKindName(item.Kind))
		}
	}
	if isKnown(elem) && !typeComparableAlone(elem) && elem.Kind != TypeNull {
		return fmt.Errorf("%s expects comparable values, got %s", name, staticKindName(elem.Kind))
	}
	if len(array.Elements) > 1 {
		base := array.Elements[0]
		for _, item := range array.Elements[1:] {
			if isKnown(base) && isKnown(item) && !typesComparable(base, item) {
				return fmt.Errorf("%s cannot compare %s and %s", name, staticKindName(base.Kind), staticKindName(item.Kind))
			}
		}
	}
	return nil
}

func typeComparableAlone(value StaticType) bool {
	return isNumberLike(value) || value.Kind == TypeString || value.Kind == TypeUnknown || value.Kind == TypeAny
}

func fieldByStaticPath(value StaticType, field string) (StaticType, bool) {
	current := value
	for _, part := range strings.Split(field, ".") {
		if part == "" {
			return StaticType{}, false
		}
		next, ok := staticField(current, part)
		if !ok {
			return StaticType{}, false
		}
		current = next
	}
	return current, true
}

func builtinLen(_ FunctionContext, args []Value) (Value, error) {
	if err := exactArgs("len", args, 1); err != nil {
		return Value{}, err
	}
	switch args[0].kind {
	case NullValue:
		return Int(0), nil
	case StringValue:
		return Int(int64(len(args[0].s))), nil
	case ArrayValue:
		return Int(int64(len(args[0].arr))), nil
	case ObjectValue:
		return Int(int64(len(args[0].obj))), nil
	default:
		return Value{}, fmt.Errorf("len expects string, array, object, or null, got %s", valueKindName(args[0].kind))
	}
}

func builtinExists(_ FunctionContext, args []Value) (Value, error) {
	if err := exactArgs("exists", args, 1); err != nil {
		return Value{}, err
	}
	return Bool(args[0].kind != NullValue), nil
}

func builtinContains(_ FunctionContext, args []Value) (Value, error) {
	if err := exactArgs("contains", args, 2); err != nil {
		return Value{}, err
	}
	ok, err := containsValue(args[0], args[1])
	if err != nil {
		return Value{}, err
	}
	return Bool(ok), nil
}

func builtinFirst(_ FunctionContext, args []Value) (Value, error) {
	items, err := listArg("first", args)
	if err != nil || len(items) == 0 {
		return Null(), err
	}
	return items[0].Clone(), nil
}

func builtinLast(_ FunctionContext, args []Value) (Value, error) {
	items, err := listArg("last", args)
	if err != nil || len(items) == 0 {
		return Null(), err
	}
	return items[len(items)-1].Clone(), nil
}

func builtinSum(_ FunctionContext, args []Value) (Value, error) {
	items, err := listArg("sum", args)
	if err != nil {
		return Value{}, err
	}
	total := numberFromInt64(0)
	for _, item := range items {
		number, ok := item.Number()
		if !ok {
			return Value{}, fmt.Errorf("sum expects numbers, got %s", valueKindName(item.kind))
		}
		total = addNumbers(total, number)
	}
	return Value{kind: NumberValue, n: total}, nil
}

func builtinAvg(ctx FunctionContext, args []Value) (Value, error) {
	items, err := listArg("avg", args)
	if err != nil || len(items) == 0 {
		return Null(), err
	}
	sum, err := builtinSum(ctx, args)
	if err != nil {
		return Value{}, err
	}
	divisor := Value{kind: NumberValue, n: numberFromInt64(int64(len(items)))}
	return arithmetic(tokenSlash, sum, divisor)
}

func builtinMin(_ FunctionContext, args []Value) (Value, error) {
	return listExtreme("min", args, false)
}

func builtinMax(_ FunctionContext, args []Value) (Value, error) {
	return listExtreme("max", args, true)
}

func builtinSort(_ FunctionContext, args []Value) (Value, error) {
	if err := exactArgs("sort", args, 1); err != nil {
		return Value{}, err
	}
	return sortList("sort", args[0], false)
}

func builtinSortDesc(_ FunctionContext, args []Value) (Value, error) {
	if err := exactArgs("sortDesc", args, 1); err != nil {
		return Value{}, err
	}
	return sortList("sortDesc", args[0], true)
}

func builtinSortBy(_ FunctionContext, args []Value) (Value, error) {
	if err := exactArgs("sortBy", args, 2); err != nil {
		return Value{}, err
	}
	return sortByField("sortBy", args[0], args[1], false)
}

func builtinSortByDesc(_ FunctionContext, args []Value) (Value, error) {
	if err := exactArgs("sortByDesc", args, 2); err != nil {
		return Value{}, err
	}
	return sortByField("sortByDesc", args[0], args[1], true)
}

func builtinKeys(_ FunctionContext, args []Value) (Value, error) {
	if err := exactArgs("keys", args, 1); err != nil {
		return Value{}, err
	}
	if args[0].kind != ObjectValue {
		return Value{}, fmt.Errorf("keys expects object, got %s", valueKindName(args[0].kind))
	}
	keys := sortKeys(args[0].obj)
	out := make([]Value, len(keys))
	for i, key := range keys {
		out[i] = String(key)
	}
	return Value{kind: ArrayValue, arr: out}, nil
}

func builtinValues(_ FunctionContext, args []Value) (Value, error) {
	if err := exactArgs("values", args, 1); err != nil {
		return Value{}, err
	}
	if args[0].kind != ObjectValue {
		return Value{}, fmt.Errorf("values expects object, got %s", valueKindName(args[0].kind))
	}
	keys := sortKeys(args[0].obj)
	out := make([]Value, len(keys))
	for i, key := range keys {
		out[i] = args[0].obj[key].Clone()
	}
	return Value{kind: ArrayValue, arr: out}, nil
}

func builtinUnique(_ FunctionContext, args []Value) (Value, error) {
	items, err := listArg("unique", args)
	if err != nil {
		return Value{}, err
	}
	out := make([]Value, 0, len(items))
	for _, item := range items {
		seen := false
		for _, existing := range out {
			if valuesEqual(existing, item) {
				seen = true
				break
			}
		}
		if !seen {
			out = append(out, item.Clone())
		}
	}
	return Value{kind: ArrayValue, arr: out}, nil
}

func builtinCompact(_ FunctionContext, args []Value) (Value, error) {
	items, err := listArg("compact", args)
	if err != nil {
		return Value{}, err
	}
	out := make([]Value, 0, len(items))
	for _, item := range items {
		if truthy(item) {
			out = append(out, item.Clone())
		}
	}
	return Value{kind: ArrayValue, arr: out}, nil
}

func builtinJoin(_ FunctionContext, args []Value) (Value, error) {
	if err := exactArgs("join", args, 2); err != nil {
		return Value{}, err
	}
	items, err := asList("join", args[0])
	if err != nil {
		return Value{}, err
	}
	sep, ok := args[1].StringValue()
	if !ok {
		return Value{}, fmt.Errorf("join separator must be string")
	}
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = FormatValue(item)
	}
	return String(strings.Join(parts, sep)), nil
}

func builtinDefault(_ FunctionContext, args []Value) (Value, error) {
	if err := exactArgs("default", args, 2); err != nil {
		return Value{}, err
	}
	if args[0].kind == NullValue {
		return args[1].Clone(), nil
	}
	return args[0].Clone(), nil
}

func exactArgs(name string, args []Value, want int) error {
	if len(args) != want {
		return fmt.Errorf("%s expects %d arguments, got %d", name, want, len(args))
	}
	return nil
}

func listArg(name string, args []Value) ([]Value, error) {
	if err := exactArgs(name, args, 1); err != nil {
		return nil, err
	}
	return asList(name, args[0])
}

func asList(name string, value Value) ([]Value, error) {
	if value.kind == NullValue {
		return nil, nil
	}
	if value.kind != ArrayValue {
		return nil, fmt.Errorf("%s expects array, got %s", name, valueKindName(value.kind))
	}
	return value.arr, nil
}

func listExtreme(name string, args []Value, max bool) (Value, error) {
	items, err := listArg(name, args)
	if err != nil || len(items) == 0 {
		return Null(), err
	}
	best := items[0]
	for _, item := range items[1:] {
		cmp, err := compareValues(item, best)
		if err != nil {
			return Value{}, fmt.Errorf("%s: %w", name, err)
		}
		if max && cmp > 0 || !max && cmp < 0 {
			best = item
		}
	}
	return best.Clone(), nil
}

func sortList(name string, value Value, desc bool) (Value, error) {
	items, err := asList(name, value)
	if err != nil {
		return Value{}, err
	}
	out := make([]Value, len(items))
	for i, item := range items {
		out[i] = item.Clone()
	}
	var sortErr error
	sort.SliceStable(out, func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		cmp, err := compareValues(out[i], out[j])
		if err != nil {
			sortErr = err
			return false
		}
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
	if sortErr != nil {
		return Value{}, fmt.Errorf("%s: %w", name, sortErr)
	}
	return Value{kind: ArrayValue, arr: out}, nil
}

func sortByField(name string, value Value, fieldValue Value, desc bool) (Value, error) {
	items, err := asList(name, value)
	if err != nil {
		return Value{}, err
	}
	field, ok := fieldValue.StringValue()
	if !ok {
		return Value{}, fmt.Errorf("%s field must be string", name)
	}
	out := make([]Value, len(items))
	for i, item := range items {
		out[i] = item.Clone()
	}
	var sortErr error
	sort.SliceStable(out, func(i, j int) bool {
		if sortErr != nil {
			return false
		}
		left, ok := fieldByPath(out[i], field)
		if !ok {
			sortErr = fmt.Errorf("missing sort field %q", field)
			return false
		}
		right, ok := fieldByPath(out[j], field)
		if !ok {
			sortErr = fmt.Errorf("missing sort field %q", field)
			return false
		}
		cmp, err := compareValues(left, right)
		if err != nil {
			sortErr = err
			return false
		}
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
	if sortErr != nil {
		return Value{}, fmt.Errorf("%s: %w", name, sortErr)
	}
	return Value{kind: ArrayValue, arr: out}, nil
}

func fieldByPath(value Value, field string) (Value, bool) {
	current := value
	for _, part := range strings.Split(field, ".") {
		if part == "" {
			return Value{}, false
		}
		next, ok := objectField(current, part)
		if !ok {
			return Value{}, false
		}
		current = next
	}
	return current, true
}

func AdaptFunction(fn any) Function {
	if direct, ok := fn.(Function); ok {
		return direct
	}
	return func(ctx FunctionContext, args []Value) (Value, error) {
		return callGoFunction(ctx, fn, args)
	}
}

func callGoFunction(ctx FunctionContext, fn any, args []Value) (Value, error) {
	value := reflect.ValueOf(fn)
	if !value.IsValid() || value.Kind() != reflect.Func {
		return Value{}, fmt.Errorf("custom function must be func, got %T", fn)
	}
	fnType := value.Type()
	in := make([]reflect.Value, 0, len(args)+1)
	argStart := 0
	if fnType.NumIn() > 0 && fnType.In(0) == reflect.TypeOf(FunctionContext{}) {
		in = append(in, reflect.ValueOf(ctx))
		argStart = 1
	}
	if fnType.IsVariadic() {
		fixed := fnType.NumIn() - 1
		if len(args)+argStart < fixed {
			return Value{}, fmt.Errorf("custom function expects at least %d arguments, got %d", fixed-argStart, len(args))
		}
	} else if len(args)+argStart != fnType.NumIn() {
		return Value{}, fmt.Errorf("custom function expects %d arguments, got %d", fnType.NumIn()-argStart, len(args))
	}
	for i, arg := range args {
		target := fnType.In(i + argStart)
		if fnType.IsVariadic() && i+argStart >= fnType.NumIn()-1 {
			target = fnType.In(fnType.NumIn() - 1).Elem()
		}
		converted, err := valueToReflect(arg, target)
		if err != nil {
			return Value{}, err
		}
		in = append(in, converted)
	}
	out := value.Call(in)
	if len(out) == 0 {
		return Null(), nil
	}
	if len(out) > 2 {
		return Value{}, fmt.Errorf("custom function returns too many values")
	}
	if len(out) == 2 {
		if !out[1].Type().Implements(errorType) {
			return Value{}, fmt.Errorf("custom function second return must be error")
		}
		if !isNilReflectValue(out[1]) {
			return Value{}, out[1].Interface().(error)
		}
	}
	if len(out) == 1 && out[0].Type().Implements(errorType) {
		if isNilReflectValue(out[0]) {
			return Null(), nil
		}
		return Value{}, out[0].Interface().(error)
	}
	return EncodeValue(out[0].Interface())
}

var errorType = reflect.TypeOf((*error)(nil)).Elem()

func isNilReflectValue(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func valueToReflect(value Value, target reflect.Type) (reflect.Value, error) {
	if target == reflect.TypeOf(Value{}) {
		return reflect.ValueOf(value.Clone()), nil
	}
	if target.Kind() == reflect.Interface && reflect.TypeOf(Value{}).Implements(target) {
		return reflect.ValueOf(value.Clone()), nil
	}
	decoded := DecodeValue(value)
	if decoded == nil {
		switch target.Kind() {
		case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
			return reflect.Zero(target), nil
		default:
			return reflect.Value{}, fmt.Errorf("cannot use null as %s", target)
		}
	}
	rv := reflect.ValueOf(decoded)
	if rv.Type().AssignableTo(target) {
		return rv, nil
	}
	if converted, ok := numericReflectValue(value, target); ok {
		return converted, nil
	}
	if target.Kind() == reflect.String && value.kind == StringValue {
		return reflect.ValueOf(value.s), nil
	}
	switch target.Kind() {
	case reflect.Slice:
		if value.kind != ArrayValue {
			return reflect.Value{}, fmt.Errorf("cannot use %s as %s", valueKindName(value.kind), target)
		}
		out := reflect.MakeSlice(target, len(value.arr), len(value.arr))
		for i, item := range value.arr {
			converted, err := valueToReflect(item, target.Elem())
			if err != nil {
				return reflect.Value{}, err
			}
			out.Index(i).Set(converted)
		}
		return out, nil
	case reflect.Array:
		if value.kind != ArrayValue {
			return reflect.Value{}, fmt.Errorf("cannot use %s as %s", valueKindName(value.kind), target)
		}
		if len(value.arr) != target.Len() {
			return reflect.Value{}, fmt.Errorf("cannot use array length %d as %s", len(value.arr), target)
		}
		out := reflect.New(target).Elem()
		for i, item := range value.arr {
			converted, err := valueToReflect(item, target.Elem())
			if err != nil {
				return reflect.Value{}, err
			}
			out.Index(i).Set(converted)
		}
		return out, nil
	case reflect.Map:
		if target.Key().Kind() != reflect.String || value.kind != ObjectValue {
			return reflect.Value{}, fmt.Errorf("cannot use %s as %s", valueKindName(value.kind), target)
		}
		out := reflect.MakeMapWithSize(target, len(value.obj))
		for key, child := range value.obj {
			converted, err := valueToReflect(child, target.Elem())
			if err != nil {
				return reflect.Value{}, err
			}
			out.SetMapIndex(reflect.ValueOf(key).Convert(target.Key()), converted)
		}
		return out, nil
	}
	return reflect.Value{}, fmt.Errorf("cannot use %s as %s", valueKindName(value.kind), target)
}

func numericReflectValue(value Value, target reflect.Type) (reflect.Value, bool) {
	number, ok := value.Number()
	if !ok {
		return reflect.Value{}, false
	}
	switch target.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if number.kind != IntegerNumber || !number.int.IsInt64() {
			return reflect.Value{}, false
		}
		out := reflect.New(target).Elem()
		if out.OverflowInt(number.int.Int64()) {
			return reflect.Value{}, false
		}
		out.SetInt(number.int.Int64())
		return out, true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if number.kind != IntegerNumber || !number.int.IsUint64() {
			return reflect.Value{}, false
		}
		out := reflect.New(target).Elem()
		if out.OverflowUint(number.int.Uint64()) {
			return reflect.Value{}, false
		}
		out.SetUint(number.int.Uint64())
		return out, true
	case reflect.Float32, reflect.Float64:
		f, _ := number.floatValue().Float64()
		out := reflect.New(target).Elem()
		if out.OverflowFloat(f) {
			return reflect.Value{}, false
		}
		out.SetFloat(f)
		return out, true
	default:
		return reflect.Value{}, false
	}
}
