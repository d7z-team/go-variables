package variables

import (
	"fmt"
	"sort"
	"strings"
)

type exprNode interface {
	eval(*evalContext) (evalResult, error)
}

type evalContext struct {
	root       Value
	current    Value
	path       Path
	opts       options
	filterMode bool
}

type evalResult struct {
	value    Value
	matches  []Match
	selected bool
	list     bool
}

type literalNode struct {
	value any
	pos   int
}

func (n literalNode) eval(*evalContext) (evalResult, error) {
	value, err := EncodeValue(n.value)
	if err != nil {
		return evalResult{}, err
	}
	return valueResult(value), nil
}

type arrayNode struct {
	items []exprNode
	pos   int
}

func (n arrayNode) eval(ctx *evalContext) (evalResult, error) {
	items := make([]Value, len(n.items))
	for i, item := range n.items {
		value, err := item.eval(ctx)
		if err != nil {
			return evalResult{}, err
		}
		items[i] = value.output()
	}
	return valueResult(Value{kind: ArrayValue, arr: items}), nil
}

type objectNode struct {
	keys  []string
	items map[string]exprNode
	pos   int
}

func (n objectNode) eval(ctx *evalContext) (evalResult, error) {
	object := make(map[string]Value, len(n.items))
	for _, key := range n.keys {
		value, err := n.items[key].eval(ctx)
		if err != nil {
			return evalResult{}, err
		}
		object[key] = value.output()
	}
	return valueResult(Value{kind: ObjectValue, obj: object}), nil
}

type identifierNode struct {
	name string
	pos  int
}

func (n identifierNode) eval(ctx *evalContext) (evalResult, error) {
	if value, ok := objectField(ctx.current, n.name); ok {
		return selectionResult([]Match{{Path: ctx.path.Child(Key(n.name)), Value: value}}), nil
	}
	if ctx.filterMode {
		return valueResult(Null()), nil
	}
	return evalResult{}, fmt.Errorf("unknown variable %q", n.name)
}

type rootNode struct{ pos int }

func (rootNode) eval(ctx *evalContext) (evalResult, error) {
	return selectionResult([]Match{{Path: Root(), Value: ctx.root}}), nil
}

type memberNode struct {
	receiver exprNode
	name     string
	optional bool
	pos      int
}

func (n memberNode) eval(ctx *evalContext) (evalResult, error) {
	receiver, err := n.receiver.eval(ctx)
	if err != nil {
		return evalResult{}, err
	}
	if receiver.selected {
		return accessMemberMatches(receiver.matches, n.name, n.optional || ctx.filterMode, receiver.list)
	}
	value, err := accessMemberValue(receiver.value, n.name, n.optional || ctx.filterMode)
	if err != nil {
		return evalResult{}, err
	}
	return valueResult(value), nil
}

type indexNode struct {
	receiver exprNode
	index    exprNode
	pos      int
}

func (n indexNode) eval(ctx *evalContext) (evalResult, error) {
	receiver, err := n.receiver.eval(ctx)
	if err != nil {
		return evalResult{}, err
	}
	index, err := n.index.eval(ctx)
	if err != nil {
		return evalResult{}, err
	}
	if receiver.selected {
		return accessIndexMatches(receiver.matches, index.output(), ctx.filterMode, receiver.list)
	}
	value, err := accessIndexValue(receiver.value, index.output(), ctx.filterMode)
	if err != nil {
		return evalResult{}, err
	}
	return valueResult(value), nil
}

type filterNode struct {
	receiver  exprNode
	predicate exprNode
	pos       int
}

func (n filterNode) eval(ctx *evalContext) (evalResult, error) {
	receiver, err := n.receiver.eval(ctx)
	if err != nil {
		return evalResult{}, err
	}
	candidates := filterCandidates(receiver)
	filtered := make([]Match, 0, len(candidates))
	for _, candidate := range candidates {
		childCtx := *ctx
		childCtx.current = candidate.Value
		childCtx.path = candidate.Path
		childCtx.filterMode = true
		result, err := n.predicate.eval(&childCtx)
		if err != nil {
			return evalResult{}, err
		}
		if truthy(result.output()) {
			filtered = append(filtered, candidate)
		}
	}
	return selectionListResult(filtered), nil
}

type unaryNode struct {
	op    tokenKind
	child exprNode
	pos   int
}

func (n unaryNode) eval(ctx *evalContext) (evalResult, error) {
	value, err := n.child.eval(ctx)
	if err != nil {
		return evalResult{}, err
	}
	out := value.output()
	switch n.op {
	case tokenNot:
		return valueResult(Bool(!truthy(out))), nil
	case tokenMinus:
		number, ok := out.Number()
		if !ok {
			return evalResult{}, fmt.Errorf("unary minus expects number, got %s", valueKindName(out.kind))
		}
		return valueResult(Value{kind: NumberValue, n: negNumber(number)}), nil
	default:
		return evalResult{}, fmt.Errorf("unsupported unary operator")
	}
}

type binaryNode struct {
	op          tokenKind
	left, right exprNode
	pos         int
}

func (n binaryNode) eval(ctx *evalContext) (evalResult, error) {
	if n.op == tokenAnd {
		left, err := n.left.eval(ctx)
		if err != nil {
			return evalResult{}, err
		}
		if !truthy(left.output()) {
			return valueResult(Bool(false)), nil
		}
		right, err := n.right.eval(ctx)
		if err != nil {
			return evalResult{}, err
		}
		return valueResult(Bool(truthy(right.output()))), nil
	}
	if n.op == tokenOr {
		left, err := n.left.eval(ctx)
		if err != nil {
			return evalResult{}, err
		}
		if truthy(left.output()) {
			return valueResult(Bool(true)), nil
		}
		right, err := n.right.eval(ctx)
		if err != nil {
			return evalResult{}, err
		}
		return valueResult(Bool(truthy(right.output()))), nil
	}
	left, err := n.left.eval(ctx)
	if err != nil {
		return evalResult{}, err
	}
	right, err := n.right.eval(ctx)
	if err != nil {
		return evalResult{}, err
	}
	value, err := evalBinary(n.op, left.output(), right.output())
	if err != nil {
		return evalResult{}, err
	}
	return valueResult(value), nil
}

type callNode struct {
	name string
	args []exprNode
	pos  int
}

func (n callNode) eval(ctx *evalContext) (evalResult, error) {
	args := make([]Value, len(n.args))
	for i, arg := range n.args {
		value, err := arg.eval(ctx)
		if err != nil {
			return evalResult{}, err
		}
		args[i] = value.output()
	}
	value, err := callFunction(ctx, n.name, args)
	if err != nil {
		return evalResult{}, err
	}
	return valueResult(value), nil
}

type methodCallNode struct {
	receiver exprNode
	name     string
	args     []exprNode
	pos      int
}

func (n methodCallNode) eval(ctx *evalContext) (evalResult, error) {
	receiver, err := n.receiver.eval(ctx)
	if err != nil {
		return evalResult{}, err
	}
	args := make([]Value, 0, len(n.args)+1)
	args = append(args, receiver.output())
	for _, arg := range n.args {
		value, err := arg.eval(ctx)
		if err != nil {
			return evalResult{}, err
		}
		args = append(args, value.output())
	}
	value, err := callFunction(ctx, n.name, args)
	if err != nil {
		return evalResult{}, err
	}
	return valueResult(value), nil
}

func callFunction(ctx *evalContext, name string, args []Value) (Value, error) {
	fnCtx := FunctionContext{Root: ctx.root, Current: ctx.current, Path: ctx.path}
	if spec, ok := builtinFunctionSpecs[name]; ok {
		return spec.Runtime(fnCtx, args)
	}
	fn, ok := ctx.opts.funcs[name]
	if !ok {
		return Value{}, fmt.Errorf("unknown function %q", name)
	}
	return fn(fnCtx, args)
}

func (v *Variables) Eval(expr Expression) (any, error) {
	value, err := v.EvalValue(expr)
	if err != nil {
		return nil, err
	}
	return DecodeValue(value), nil
}

func (v *Variables) EvalValue(expr Expression) (Value, error) {
	v.mu.RLock()
	ctx := evalContext{root: v.root, current: v.root, path: Root(), opts: v.opts}
	result, err := expr.root.eval(&ctx)
	v.mu.RUnlock()
	if err != nil {
		return Value{}, err
	}
	return result.output().Clone(), nil
}

func (v *Variables) ParseExpression(src string) (Expression, error) {
	return v.CompileExpression(src)
}

func (v *Variables) CompileExpression(src string, opts ...CompileOption) (Expression, error) {
	v.mu.RLock()
	root := v.root.Clone()
	funcs := make(map[string]FunctionSpec, len(v.opts.functionSpecs))
	for name, spec := range v.opts.functionSpecs {
		funcs[name] = spec
	}
	v.mu.RUnlock()

	compileOpts := []CompileOption{WithRootValue(root), WithFunctionSpecs(funcs)}
	compileOpts = append(compileOpts, opts...)
	return CompileExpression(src, compileOpts...)
}

func (v *Variables) EvalString(src string) (any, error) {
	expr, err := v.ParseExpression(src)
	if err != nil {
		return nil, err
	}
	return v.Eval(expr)
}

func (v *Variables) Select(expr Expression) ([]Match, error) {
	v.mu.RLock()
	ctx := evalContext{root: v.root, current: v.root, path: Root(), opts: v.opts}
	result, err := expr.root.eval(&ctx)
	v.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	if !result.selected {
		return nil, fmt.Errorf("expression %q does not select tree nodes", expr.source)
	}
	matches := make([]Match, len(result.matches))
	for i, match := range result.matches {
		matches[i] = Match{Path: match.Path, Value: match.Value.Clone()}
	}
	return matches, nil
}

func (v *Variables) SelectString(src string) ([]Match, error) {
	expr, err := v.ParseExpression(src)
	if err != nil {
		return nil, err
	}
	return v.Select(expr)
}

func (v *Variables) SelectValues(expr Expression) ([]any, error) {
	matches, err := v.Select(expr)
	if err != nil {
		return nil, err
	}
	values := make([]any, len(matches))
	for i, match := range matches {
		values[i] = DecodeValue(match.Value)
	}
	return values, nil
}

func (v *Variables) First(expr Expression) (Match, bool, error) {
	matches, err := v.Select(expr)
	if err != nil {
		return Match{}, false, err
	}
	if len(matches) == 0 {
		return Match{}, false, nil
	}
	return matches[0], true, nil
}

func (v *Variables) Count(expr Expression) (int, error) {
	matches, err := v.Select(expr)
	if err != nil {
		return 0, err
	}
	return len(matches), nil
}

func valueResult(value Value) evalResult {
	return evalResult{value: value}
}

func selectionResult(matches []Match) evalResult {
	return evalResult{matches: matches, selected: true}
}

func selectionListResult(matches []Match) evalResult {
	return evalResult{matches: matches, selected: true, list: true}
}

func (r evalResult) output() Value {
	if !r.selected {
		return r.value
	}
	if len(r.matches) == 0 {
		return Value{kind: ArrayValue, arr: []Value{}}
	}
	if len(r.matches) == 1 && !r.list {
		return r.matches[0].Value
	}
	values := make([]Value, len(r.matches))
	for i, match := range r.matches {
		values[i] = match.Value
	}
	return Value{kind: ArrayValue, arr: values}
}

func objectField(value Value, name string) (Value, bool) {
	if value.kind != ObjectValue {
		return Value{}, false
	}
	child, ok := value.obj[name]
	return child, ok
}

func accessMemberMatches(matches []Match, name string, optional bool, forceList bool) (evalResult, error) {
	out := make([]Match, 0, len(matches))
	projected := forceList
	for _, match := range matches {
		if match.Value.kind == ArrayValue {
			projected = true
			for i, item := range match.Value.arr {
				childPath := match.Path.Child(Index(i))
				value, ok := objectField(item, name)
				if !ok {
					if optional {
						out = append(out, Match{Path: childPath.Child(Key(name)), Value: Null()})
						continue
					}
					return evalResult{}, fmt.Errorf("missing field %q at %s", name, childPath.String())
				}
				out = append(out, Match{Path: childPath.Child(Key(name)), Value: value})
			}
			continue
		}
		value, ok := objectField(match.Value, name)
		if !ok {
			if optional {
				out = append(out, Match{Path: match.Path.Child(Key(name)), Value: Null()})
				continue
			}
			return evalResult{}, fmt.Errorf("missing field %q at %s", name, match.Path.String())
		}
		out = append(out, Match{Path: match.Path.Child(Key(name)), Value: value})
	}
	return evalResult{matches: out, selected: true, list: projected}, nil
}

func accessMemberValue(value Value, name string, optional bool) (Value, error) {
	if value.kind == ArrayValue {
		out := make([]Value, 0, len(value.arr))
		for _, item := range value.arr {
			child, ok := objectField(item, name)
			if !ok {
				if optional {
					out = append(out, Null())
					continue
				}
				return Value{}, fmt.Errorf("missing field %q", name)
			}
			out = append(out, child)
		}
		return Value{kind: ArrayValue, arr: out}, nil
	}
	child, ok := objectField(value, name)
	if !ok {
		if optional {
			return Null(), nil
		}
		return Value{}, fmt.Errorf("missing field %q", name)
	}
	return child, nil
}

func accessIndexMatches(matches []Match, indexValue Value, optional bool, forceList bool) (evalResult, error) {
	out := make([]Match, 0, len(matches))
	for _, match := range matches {
		value, path, ok, err := accessIndex(match.Value, match.Path, indexValue)
		if err != nil {
			return evalResult{}, err
		}
		if !ok {
			if optional {
				out = append(out, Match{Path: match.Path, Value: Null()})
				continue
			}
			return evalResult{}, fmt.Errorf("index not found at %s", match.Path.String())
		}
		out = append(out, Match{Path: path, Value: value})
	}
	return evalResult{matches: out, selected: true, list: forceList && len(out) != 1}, nil
}

func accessIndexValue(value Value, indexValue Value, optional bool) (Value, error) {
	value, _, ok, err := accessIndex(value, Root(), indexValue)
	if err != nil {
		return Value{}, err
	}
	if !ok {
		if optional {
			return Null(), nil
		}
		return Value{}, ErrIndexOutOfRange
	}
	return value, nil
}

func accessIndex(value Value, base Path, indexValue Value) (Value, Path, bool, error) {
	switch value.kind {
	case ArrayValue:
		index, ok := valueIndex(indexValue)
		if !ok {
			return Value{}, base, false, fmt.Errorf("array index must be integer")
		}
		index = normalizeIndex(index, len(value.arr))
		if index < 0 || index >= len(value.arr) {
			return Value{}, base, false, nil
		}
		return value.arr[index], base.Child(Index(index)), true, nil
	case ObjectValue:
		key, ok := indexValue.StringValue()
		if !ok {
			return Value{}, base, false, fmt.Errorf("object index must be string")
		}
		child, ok := value.obj[key]
		if !ok {
			return Value{}, base, false, nil
		}
		return child, base.Child(Key(key)), true, nil
	default:
		return Value{}, base, false, nil
	}
}

func filterCandidates(result evalResult) []Match {
	if result.selected {
		var out []Match
		for _, match := range result.matches {
			if match.Value.kind == ArrayValue {
				for i, item := range match.Value.arr {
					out = append(out, Match{Path: match.Path.Child(Index(i)), Value: item})
				}
			} else {
				out = append(out, match)
			}
		}
		return out
	}
	if result.value.kind != ArrayValue {
		return nil
	}
	out := make([]Match, len(result.value.arr))
	for i, item := range result.value.arr {
		out[i] = Match{Path: Root().Child(Index(i)), Value: item}
	}
	return out
}

func evalBinary(op tokenKind, left Value, right Value) (Value, error) {
	switch op {
	case tokenEqual:
		return Bool(valuesEqual(left, right)), nil
	case tokenNotEqual:
		return Bool(!valuesEqual(left, right)), nil
	case tokenGreater, tokenGreaterEqual, tokenLess, tokenLessEqual:
		cmp, err := compareValues(left, right)
		if err != nil {
			return Value{}, err
		}
		switch op {
		case tokenGreater:
			return Bool(cmp > 0), nil
		case tokenGreaterEqual:
			return Bool(cmp >= 0), nil
		case tokenLess:
			return Bool(cmp < 0), nil
		default:
			return Bool(cmp <= 0), nil
		}
	case tokenIn:
		ok, err := containsValue(right, left)
		if err != nil {
			return Value{}, err
		}
		return Bool(ok), nil
	case tokenPlus, tokenMinus, tokenStar, tokenSlash, tokenPercent:
		return arithmetic(op, left, right)
	default:
		return Value{}, fmt.Errorf("unsupported binary operator")
	}
}

func arithmetic(op tokenKind, left Value, right Value) (Value, error) {
	if op == tokenPlus && left.kind == StringValue {
		if right.kind != StringValue {
			return Value{}, fmt.Errorf("cannot add string and %s", valueKindName(right.kind))
		}
		return String(left.s + right.s), nil
	}
	l, lok := left.Number()
	r, rok := right.Number()
	if !lok || !rok {
		return Value{}, fmt.Errorf("arithmetic expects numbers, got %s and %s", valueKindName(left.kind), valueKindName(right.kind))
	}
	var out Number
	var err error
	switch op {
	case tokenPlus:
		out = addNumbers(l, r)
	case tokenMinus:
		out = subNumbers(l, r)
	case tokenStar:
		out = mulNumbers(l, r)
	case tokenSlash:
		out, err = divNumbers(l, r)
	case tokenPercent:
		out, err = modNumbers(l, r)
	default:
		err = fmt.Errorf("unsupported arithmetic operator")
	}
	if err != nil {
		return Value{}, err
	}
	return Value{kind: NumberValue, n: out}, nil
}

func valuesEqual(left Value, right Value) bool {
	if left.kind == NumberValue && right.kind == NumberValue {
		return compareNumbers(left.n, right.n) == 0
	}
	if left.kind != right.kind {
		return false
	}
	switch left.kind {
	case NullValue:
		return true
	case BoolValue:
		return left.b == right.b
	case StringValue:
		return left.s == right.s
	case ObjectValue, ArrayValue:
		return FormatValue(left) == FormatValue(right)
	default:
		return false
	}
}

func truthy(value Value) bool {
	switch value.kind {
	case NullValue:
		return false
	case BoolValue:
		return value.b
	case StringValue:
		return value.s != ""
	case ArrayValue:
		return len(value.arr) > 0
	case ObjectValue:
		return len(value.obj) > 0
	case NumberValue:
		return !value.n.isZero()
	default:
		return false
	}
}

func containsValue(container Value, needle Value) (bool, error) {
	switch container.kind {
	case StringValue:
		part, ok := needle.StringValue()
		if !ok {
			return false, fmt.Errorf("string contains expects string needle")
		}
		return strings.Contains(container.s, part), nil
	case ArrayValue:
		for _, item := range container.arr {
			if valuesEqual(item, needle) {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("in expects array or string, got %s", valueKindName(container.kind))
	}
}

func compareValues(left Value, right Value) (int, error) {
	if left.kind == NumberValue && right.kind == NumberValue {
		return compareNumbers(left.n, right.n), nil
	}
	if left.kind == StringValue && right.kind == StringValue {
		switch {
		case left.s < right.s:
			return -1, nil
		case left.s > right.s:
			return 1, nil
		default:
			return 0, nil
		}
	}
	return 0, fmt.Errorf("cannot compare %s and %s", valueKindName(left.kind), valueKindName(right.kind))
}

func valueIndex(value Value) (int, bool) {
	number, ok := value.Number()
	if !ok || number.kind != IntegerNumber || !number.int.IsInt64() {
		return 0, false
	}
	i := number.int.Int64()
	if int64(int(i)) != i {
		return 0, false
	}
	return int(i), true
}

func sortKeys(node map[string]Value) []string {
	keys := make([]string, 0, len(node))
	for key := range node {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func normalizeIndex(index int, length int) int {
	if index < 0 {
		return length + index
	}
	return index
}

func valueKindName(kind ValueKind) string {
	switch kind {
	case NullValue:
		return "null"
	case BoolValue:
		return "bool"
	case StringValue:
		return "string"
	case NumberValue:
		return "number"
	case ObjectValue:
		return "object"
	case ArrayValue:
		return "array"
	default:
		return "unknown"
	}
}
