package variables

import (
	"math/big"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpressionEvalArithmeticComparisonAndLiterals(t *testing.T) {
	v := expressionFixture(t)

	value, err := v.Eval(MustExpression(`1 + 2 * 3`))
	require.NoError(t, err)
	require.Equal(t, wantInt(7), value)

	value, err = v.Eval(MustExpression(`servers[0].cpu * 2 >= 4 && "us" in ["us", "eu"]`))
	require.NoError(t, err)
	require.Equal(t, true, value)

	value, err = v.Eval(MustExpression(`{"name": app.name, "hosts": servers.host}`))
	require.NoError(t, err)
	require.Equal(t, map[string]any{"name": "demo", "hosts": []any{"a", "b", "c"}}, value)
}

func TestExpressionEvalReportsTypeAndMathErrors(t *testing.T) {
	v := expressionFixture(t)

	_, err := ParseExpression(`"a" + 1`)
	require.Error(t, err)

	_, err = ParseExpression(`1 / 0`)
	require.Error(t, err)

	_, err = ParseExpression(`1 % 0`)
	require.Error(t, err)

	_, err = v.Eval(MustExpression(`servers.host > 1`))
	require.Error(t, err)
}

func TestExpressionEvalDecimalMathAndNumberEquality(t *testing.T) {
	v := expressionFixture(t)

	value, err := v.Eval(MustExpression(`1.5 + 2`))
	require.NoError(t, err)
	requireBigFloatEqual(t, "3.5", value)

	value, err = v.Eval(MustExpression(`1 == 1.0`))
	require.NoError(t, err)
	require.Equal(t, true, value)

	value, err = v.Eval(MustExpression(`9223372036854775808 + 1`))
	require.NoError(t, err)
	require.Equal(t, "9223372036854775809", value.(*big.Int).String())

	value, err = v.Eval(MustExpression(`1.25e2 + 0.5`))
	require.NoError(t, err)
	requireBigFloatEqual(t, "125.5", value)
}

func TestPrecompiledExpressionCanBeReusedConcurrently(t *testing.T) {
	v := expressionFixture(t)
	expr := MustExpression(`sum(servers?[enabled].cpu)`)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			value, err := v.Eval(expr)
			require.NoError(t, err)
			require.Equal(t, wantInt(10), value)
		}()
	}
	wg.Wait()
}
