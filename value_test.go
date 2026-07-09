package variables

import (
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func wantInt(v int64) *big.Int {
	return big.NewInt(v)
}

func wantFloat(src string) *big.Float {
	value, _, err := big.ParseFloat(src, 10, numberPrecision, big.ToNearestEven)
	if err != nil {
		panic(err)
	}
	return value
}

func requireBigFloatEqual(t *testing.T, want string, got any) {
	t.Helper()
	actual, ok := got.(*big.Float)
	require.Truef(t, ok, "expected *big.Float, got %T", got)
	require.Equal(t, wantFloat(want).Text('g', -1), actual.Text('g', -1))
}

func requireDecodedEqual(t *testing.T, want any, got any) {
	t.Helper()
	if wantFloat, ok := want.(*big.Float); ok {
		gotFloat, ok := got.(*big.Float)
		require.Truef(t, ok, "expected *big.Float, got %T", got)
		require.Equal(t, wantFloat.Text('g', -1), gotFloat.Text('g', -1))
		return
	}
	require.Equal(t, want, got)
}

func TestValueRejectsInvalidTreeInputs(t *testing.T) {
	type config struct {
		Name string
	}

	v := New()
	require.ErrorIs(t, v.Set(MustPath("struct"), config{Name: "bad"}), ErrInvalidValue)
	require.ErrorIs(t, v.Set(MustPath("pointer"), &config{Name: "bad"}), ErrInvalidValue)
	require.ErrorIs(t, v.Set(MustPath("map"), map[int]string{1: "bad"}), ErrInvalidValue)
	require.ErrorIs(t, v.Set(MustPath("func"), func() {}), ErrInvalidValue)
	require.ErrorIs(t, v.Set(MustPath("nan"), math.NaN()), ErrInvalidNumber)
	require.ErrorIs(t, v.Set(MustPath("inf"), math.Inf(1)), ErrInvalidNumber)

	require.NoError(t, v.Set(MustPath("items"), []any{}))
	require.ErrorIs(t, v.Append(MustPath("items"), &config{Name: "bad"}), ErrInvalidValue)
}

func TestValueAcceptsExplicitBigNumbersAndClones(t *testing.T) {
	v := New()
	bigInteger := new(big.Int)
	bigInteger.SetString("123456789012345678901234567890", 10)
	bigDecimal := wantFloat("1.234567890123456789")

	require.NoError(t, v.Set(MustPath("n"), bigInteger))
	require.NoError(t, v.Set(MustPath("f"), bigDecimal))

	bigInteger.SetInt64(1)
	bigDecimal.SetInt64(1)

	value, ok := v.Get(MustPath("n"))
	require.True(t, ok)
	require.Equal(t, "123456789012345678901234567890", value.(*big.Int).String())

	value, ok = v.Get(MustPath("f"))
	require.True(t, ok)
	requireBigFloatEqual(t, "1.234567890123456789", value)
}

func TestValueAPIsReturnClones(t *testing.T) {
	v := New()
	require.NoError(t, v.SetValue(MustPath("items"), Array([]Value{
		Object(map[string]Value{"name": String("one")}),
	})))

	value, ok := v.GetValue(MustPath("items"))
	require.True(t, ok)
	value.arr[0].obj["name"] = String("changed")

	value, ok = v.GetValue(MustPath("items[0].name"))
	require.True(t, ok)
	require.Equal(t, String("one"), value)

	snapshot := v.SnapshotValue()
	snapshot.obj["items"].arr[0].obj["name"] = String("changed")
	value, ok = v.GetValue(MustPath("items[0].name"))
	require.True(t, ok)
	require.Equal(t, String("one"), value)
}
