package variables

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strconv"
)

const numberPrecision uint = 256

var (
	ErrInvalidValue  = errors.New("invalid value")
	ErrInvalidNumber = errors.New("invalid number")
)

type ValueKind uint8

const (
	NullValue ValueKind = iota
	BoolValue
	StringValue
	NumberValue
	ObjectValue
	ArrayValue
)

type NumberKind uint8

const (
	IntegerNumber NumberKind = iota
	FloatNumber
)

type Number struct {
	kind  NumberKind
	int   *big.Int
	float *big.Float
}

type Value struct {
	kind ValueKind
	b    bool
	s    string
	n    Number
	obj  map[string]Value
	arr  []Value
}

func Null() Value { return Value{kind: NullValue} }

func Bool(v bool) Value { return Value{kind: BoolValue, b: v} }

func String(v string) Value { return Value{kind: StringValue, s: v} }

func Int(v int64) Value { return BigInt(big.NewInt(v)) }

func Uint(v uint64) Value { return BigInt(new(big.Int).SetUint64(v)) }

func BigInt(v *big.Int) Value {
	if v == nil {
		return Null()
	}
	return Value{kind: NumberValue, n: Number{kind: IntegerNumber, int: new(big.Int).Set(v)}}
}

func Float(v float64) (Value, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return Value{}, fmt.Errorf("%w: non-finite float", ErrInvalidNumber)
	}
	return BigFloat(newBigFloat(v)), nil
}

func BigFloat(v *big.Float) Value {
	if v == nil {
		return Null()
	}
	return Value{kind: NumberValue, n: Number{kind: FloatNumber, float: cloneBigFloat(v)}}
}

func Object(v map[string]Value) Value {
	out := make(map[string]Value, len(v))
	for key, child := range v {
		out[key] = child.Clone()
	}
	return Value{kind: ObjectValue, obj: out}
}

func Array(v []Value) Value {
	out := make([]Value, len(v))
	for i, child := range v {
		out[i] = child.Clone()
	}
	return Value{kind: ArrayValue, arr: out}
}

func (v Value) Kind() ValueKind { return v.kind }

func (v Value) Bool() (bool, bool) {
	return v.b, v.kind == BoolValue
}

func (v Value) StringValue() (string, bool) {
	return v.s, v.kind == StringValue
}

func (v Value) Number() (Number, bool) {
	if v.kind != NumberValue {
		return Number{}, false
	}
	return v.n.Clone(), true
}

func (v Value) Object() (map[string]Value, bool) {
	if v.kind != ObjectValue {
		return nil, false
	}
	out := make(map[string]Value, len(v.obj))
	for key, child := range v.obj {
		out[key] = child.Clone()
	}
	return out, true
}

func (v Value) Array() ([]Value, bool) {
	if v.kind != ArrayValue {
		return nil, false
	}
	out := make([]Value, len(v.arr))
	for i, child := range v.arr {
		out[i] = child.Clone()
	}
	return out, true
}

func (v Value) Clone() Value {
	switch v.kind {
	case NumberValue:
		v.n = v.n.Clone()
	case ObjectValue:
		out := make(map[string]Value, len(v.obj))
		for key, child := range v.obj {
			out[key] = child.Clone()
		}
		v.obj = out
	case ArrayValue:
		out := make([]Value, len(v.arr))
		for i, child := range v.arr {
			out[i] = child.Clone()
		}
		v.arr = out
	}
	return v
}

func (n Number) Clone() Number {
	switch n.kind {
	case IntegerNumber:
		if n.int != nil {
			n.int = new(big.Int).Set(n.int)
		}
	case FloatNumber:
		n.float = cloneBigFloat(n.float)
	}
	return n
}

func (n Number) Text() string {
	switch n.kind {
	case IntegerNumber:
		if n.int == nil {
			return "0"
		}
		return n.int.String()
	case FloatNumber:
		if n.float == nil {
			return "0"
		}
		return n.float.Text('g', -1)
	default:
		return "0"
	}
}

func EncodeValue(src any) (Value, error) {
	if value, ok := src.(Value); ok {
		return value.Clone(), nil
	}
	return encodeReflectValue(reflect.ValueOf(src))
}

func encodeReflectValue(src reflect.Value) (Value, error) {
	if !src.IsValid() {
		return Null(), nil
	}
	if src.Kind() == reflect.Interface {
		if src.IsNil() {
			return Null(), nil
		}
		return encodeReflectValue(src.Elem())
	}
	if src.CanInterface() {
		switch value := src.Interface().(type) {
		case json.Number:
			return parseJSONNumber(value)
		case *big.Int:
			return BigInt(value), nil
		case big.Int:
			return BigInt(&value), nil
		case *big.Float:
			return BigFloat(value), nil
		case big.Float:
			return BigFloat(&value), nil
		}
	}
	if src.Kind() == reflect.Pointer {
		if src.IsNil() {
			return Null(), nil
		}
		return Value{}, fmt.Errorf("%w: pointer %s is not allowed", ErrInvalidValue, src.Type())
	}
	switch src.Kind() {
	case reflect.Bool:
		return Bool(src.Bool()), nil
	case reflect.String:
		return String(src.String()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return BigInt(big.NewInt(src.Int())), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return BigInt(new(big.Int).SetUint64(src.Uint())), nil
	case reflect.Float32, reflect.Float64:
		return Float(src.Float())
	case reflect.Map:
		if src.Type().Key().Kind() != reflect.String {
			return Value{}, fmt.Errorf("%w: map key %s is not allowed", ErrInvalidValue, src.Type().Key())
		}
		if src.IsNil() {
			return Null(), nil
		}
		out := make(map[string]Value, src.Len())
		keys := src.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
		for _, key := range keys {
			child, err := encodeReflectValue(src.MapIndex(key))
			if err != nil {
				return Value{}, err
			}
			out[key.String()] = child
		}
		return Value{kind: ObjectValue, obj: out}, nil
	case reflect.Slice, reflect.Array:
		if src.Kind() == reflect.Slice && src.IsNil() {
			return Null(), nil
		}
		out := make([]Value, src.Len())
		for i := 0; i < src.Len(); i++ {
			child, err := encodeReflectValue(src.Index(i))
			if err != nil {
				return Value{}, err
			}
			out[i] = child
		}
		return Value{kind: ArrayValue, arr: out}, nil
	case reflect.Struct:
		return Value{}, fmt.Errorf("%w: struct %s is not allowed", ErrInvalidValue, src.Type())
	case reflect.Func, reflect.Chan, reflect.UnsafePointer, reflect.Complex64, reflect.Complex128:
		return Value{}, fmt.Errorf("%w: %s is not allowed", ErrInvalidValue, src.Type())
	default:
		return Value{}, fmt.Errorf("%w: %s is not allowed", ErrInvalidValue, src.Type())
	}
}

func parseJSONNumber(src json.Number) (Value, error) {
	raw := src.String()
	if i, ok := new(big.Int).SetString(raw, 10); ok {
		return BigInt(i), nil
	}
	f, _, err := big.ParseFloat(raw, 10, numberPrecision, big.ToNearestEven)
	if err != nil {
		return Value{}, fmt.Errorf("%w: %q", ErrInvalidNumber, raw)
	}
	return BigFloat(f), nil
}

func DecodeValue(v Value) any {
	switch v.kind {
	case NullValue:
		return nil
	case BoolValue:
		return v.b
	case StringValue:
		return v.s
	case NumberValue:
		switch v.n.kind {
		case IntegerNumber:
			if v.n.int == nil {
				return big.NewInt(0)
			}
			return new(big.Int).Set(v.n.int)
		case FloatNumber:
			return cloneBigFloat(v.n.float)
		default:
			return nil
		}
	case ObjectValue:
		out := make(map[string]any, len(v.obj))
		for key, child := range v.obj {
			out[key] = DecodeValue(child)
		}
		return out
	case ArrayValue:
		out := make([]any, len(v.arr))
		for i, child := range v.arr {
			out[i] = DecodeValue(child)
		}
		return out
	default:
		return nil
	}
}

func JSONValue(v Value) any {
	switch v.kind {
	case NullValue:
		return nil
	case BoolValue:
		return v.b
	case StringValue:
		return v.s
	case NumberValue:
		return json.Number(v.n.Text())
	case ObjectValue:
		out := make(map[string]any, len(v.obj))
		for key, child := range v.obj {
			out[key] = JSONValue(child)
		}
		return out
	case ArrayValue:
		out := make([]any, len(v.arr))
		for i, child := range v.arr {
			out[i] = JSONValue(child)
		}
		return out
	default:
		return nil
	}
}

func FormatValue(v Value) string {
	switch v.kind {
	case NullValue:
		return ""
	case BoolValue:
		return strconv.FormatBool(v.b)
	case StringValue:
		return v.s
	case NumberValue:
		return v.n.Text()
	case ObjectValue, ArrayValue:
		data, err := json.Marshal(JSONValue(v))
		if err != nil {
			return fmt.Sprint(DecodeValue(v))
		}
		return string(data)
	default:
		return ""
	}
}

func cloneBigFloat(src *big.Float) *big.Float {
	if src == nil {
		return new(big.Float).SetPrec(numberPrecision)
	}
	return new(big.Float).SetPrec(numberPrecision).SetMode(big.ToNearestEven).Set(src)
}

func newBigFloat(v float64) *big.Float {
	return new(big.Float).SetPrec(numberPrecision).SetMode(big.ToNearestEven).SetFloat64(v)
}

func numberFromInt64(v int64) Number {
	return Number{kind: IntegerNumber, int: big.NewInt(v)}
}
