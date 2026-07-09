package variables

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompileExpressionUsesCurrentTreeSchema(t *testing.T) {
	v := expressionFixture(t)

	_, err := v.CompileExpression(`app.missing`)
	require.Error(t, err)

	_, err = v.CompileExpression(`app?.missing`)
	require.NoError(t, err)

	_, err = v.CompileExpression(`servers["bad"]`)
	require.Error(t, err)

	_, err = v.CompileExpression(`servers[10]`)
	require.Error(t, err)

	_, err = v.CompileExpression(`servers[2].host`)
	require.NoError(t, err)

	_, err = v.CompileExpression(`servers[-1].host`)
	require.NoError(t, err)

	_, err = v.CompileExpression(`servers[-4]`)
	require.Error(t, err)

	_, err = v.CompileExpression(`app[1]`)
	require.Error(t, err)

	_, err = v.CompileExpression(`sum(servers.host)`)
	require.Error(t, err)

	_, err = v.CompileExpression(`sortBy(servers, "missing")`)
	require.Error(t, err)
}

func TestCompileExpressionChecksHeterogeneousArrayFields(t *testing.T) {
	v := New()
	require.NoError(t, v.Set(MustPath("items"), []any{
		map[string]any{"name": "a"},
		map[string]any{"cpu": 1},
	}))

	_, err := v.CompileExpression(`items.name`)
	require.Error(t, err)

	_, err = v.CompileExpression(`items?.name`)
	require.NoError(t, err)
}

func TestCompileExpressionKeepsUnknownsDynamicUnlessStrict(t *testing.T) {
	_, err := CompileExpression(`missing + 1`)
	require.NoError(t, err)

	_, err = CompileExpression(`missing + 1`, WithStrictTypes())
	require.Error(t, err)

	_, err = CompileExpression(`items?[enabled].name`)
	require.NoError(t, err)
}

func TestCompileExpressionChecksMethodCalls(t *testing.T) {
	v := expressionFixture(t)

	_, err := v.CompileExpression(`servers.cpu.sum()`)
	require.NoError(t, err)

	_, err = v.CompileExpression(`servers.cpu.sum("extra")`)
	require.Error(t, err)

	_, err = v.CompileExpression(`servers.host.sum()`)
	require.Error(t, err)
}

func TestCompileExpressionChecksCustomGoFunctionSignatures(t *testing.T) {
	v := New(WithGoFunction("repeat", func(value string, count int) string {
		return fmt.Sprintf("%s:%d", value, count)
	}))
	require.NoError(t, v.Set(MustPath("name"), "dragon"))

	_, err := v.CompileExpression(`repeat(name, 2)`)
	require.NoError(t, err)

	_, err = v.CompileExpression(`repeat(2, name)`)
	require.Error(t, err)

	_, err = v.CompileExpression(`repeat(name)`)
	require.Error(t, err)
}

func TestCustomGoFunctionConvertsTypedCollections(t *testing.T) {
	v := expressionFixture(t).Clone(
		WithGoFunction("joinStrings", func(values []string) string {
			return fmt.Sprintf("%s:%d", values[0], len(values))
		}),
		WithGoFunction("region", func(app map[string]string) string {
			return app["defaultRegion"]
		}),
	)

	value, err := v.EvalString(`joinStrings(servers.host)`)
	require.NoError(t, err)
	require.Equal(t, "a:3", value)

	value, err = v.EvalString(`region(app)`)
	require.NoError(t, err)
	require.Equal(t, "us", value)

	_, err = v.CompileExpression(`joinStrings(servers.cpu)`)
	require.Error(t, err)

	_, err = v.CompileExpression(`region({"ok": 1})`)
	require.Error(t, err)
}

func TestCompileExpressionAllowsDynamicValueFunctions(t *testing.T) {
	v := New(WithFunction("dynamic", Function(func(_ FunctionContext, _ []Value) (Value, error) {
		return String("ok"), nil
	})))

	_, err := v.CompileExpression(`dynamic(1, "x", null)`)
	require.NoError(t, err)
}

func TestCompileExpressionUsesExplicitSchemaAndFunctionSpecs(t *testing.T) {
	schema := StaticType{Kind: TypeObject, Fields: map[string]StaticType{
		"app": {Kind: TypeObject, Fields: map[string]StaticType{
			"port": integerType(),
		}},
	}}
	specs := map[string]FunctionSpec{
		"inc": {
			MinArgs:    1,
			MaxArgs:    1,
			Params:     []StaticType{integerType()},
			ReturnType: integerType(),
			Runtime: func(_ FunctionContext, args []Value) (Value, error) {
				number, _ := args[0].Number()
				return Value{kind: NumberValue, n: addNumbers(number, numberFromInt64(1))}, nil
			},
		},
	}

	_, err := CompileExpression(`inc(app.port)`, WithRootSchema(schema), WithFunctionSpecs(specs))
	require.NoError(t, err)

	_, err = CompileExpression(`inc("bad")`, WithRootSchema(schema), WithFunctionSpecs(specs))
	require.Error(t, err)
}

func TestWithTypedFunctionAddsStaticChecks(t *testing.T) {
	v := New(WithTypedFunction("tag", FunctionSpec{
		MinArgs:    1,
		MaxArgs:    1,
		Params:     []StaticType{stringType()},
		ReturnType: stringType(),
		Runtime: func(_ FunctionContext, args []Value) (Value, error) {
			value, _ := args[0].StringValue()
			return String("tag:" + value), nil
		},
	}))

	value, err := v.EvalString(`tag("ok")`)
	require.NoError(t, err)
	require.Equal(t, "tag:ok", value)

	_, err = v.CompileExpression(`tag(1)`)
	require.Error(t, err)
}

func TestWithTypedFunctionRequiresRuntime(t *testing.T) {
	require.Panics(t, func() {
		_ = New(WithTypedFunction("broken", FunctionSpec{
			MinArgs:    0,
			MaxArgs:    0,
			ReturnType: stringType(),
		}))
	})
}
