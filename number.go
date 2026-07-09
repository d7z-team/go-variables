package variables

import (
	"fmt"
	"math/big"
)

func compareNumbers(left Number, right Number) int {
	if left.kind == IntegerNumber && right.kind == IntegerNumber {
		return left.intValue().Cmp(right.intValue())
	}
	return left.floatValue().Cmp(right.floatValue())
}

func addNumbers(left Number, right Number) Number {
	if left.kind == IntegerNumber && right.kind == IntegerNumber {
		return Number{kind: IntegerNumber, int: new(big.Int).Add(left.intValue(), right.intValue())}
	}
	return Number{kind: FloatNumber, float: newFloat().Add(left.floatValue(), right.floatValue())}
}

func subNumbers(left Number, right Number) Number {
	if left.kind == IntegerNumber && right.kind == IntegerNumber {
		return Number{kind: IntegerNumber, int: new(big.Int).Sub(left.intValue(), right.intValue())}
	}
	return Number{kind: FloatNumber, float: newFloat().Sub(left.floatValue(), right.floatValue())}
}

func mulNumbers(left Number, right Number) Number {
	if left.kind == IntegerNumber && right.kind == IntegerNumber {
		return Number{kind: IntegerNumber, int: new(big.Int).Mul(left.intValue(), right.intValue())}
	}
	return Number{kind: FloatNumber, float: newFloat().Mul(left.floatValue(), right.floatValue())}
}

func divNumbers(left Number, right Number) (Number, error) {
	if right.isZero() {
		return Number{}, fmt.Errorf("division by zero")
	}
	return Number{kind: FloatNumber, float: newFloat().Quo(left.floatValue(), right.floatValue())}, nil
}

func modNumbers(left Number, right Number) (Number, error) {
	if left.kind != IntegerNumber || right.kind != IntegerNumber {
		return Number{}, fmt.Errorf("modulo expects integers")
	}
	if right.isZero() {
		return Number{}, fmt.Errorf("modulo by zero")
	}
	return Number{kind: IntegerNumber, int: new(big.Int).Mod(left.intValue(), right.intValue())}, nil
}

func negNumber(value Number) Number {
	switch value.kind {
	case IntegerNumber:
		return Number{kind: IntegerNumber, int: new(big.Int).Neg(value.intValue())}
	case FloatNumber:
		return Number{kind: FloatNumber, float: newFloat().Neg(value.floatValue())}
	default:
		return numberFromInt64(0)
	}
}

func (n Number) intValue() *big.Int {
	if n.int == nil {
		return big.NewInt(0)
	}
	return n.int
}

func (n Number) floatValue() *big.Float {
	if n.kind == IntegerNumber {
		return newFloat().SetInt(n.intValue())
	}
	if n.float == nil {
		return newFloat()
	}
	return cloneBigFloat(n.float)
}

func (n Number) isZero() bool {
	switch n.kind {
	case IntegerNumber:
		return n.intValue().Sign() == 0
	case FloatNumber:
		return n.floatValue().Sign() == 0
	default:
		return true
	}
}

func newFloat() *big.Float {
	return new(big.Float).SetPrec(numberPrecision).SetMode(big.ToNearestEven)
}
