package variables

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpressionBuiltinCollectionFunctions(t *testing.T) {
	v := expressionFixture(t)

	tests := map[string]any{
		"count(servers?[enabled])":            wantInt(2),
		"contains(servers.host, \"b\")":       true,
		"first(servers.host)":                 "a",
		"last(servers.host)":                  "c",
		"sum(servers.cpu)":                    wantInt(14),
		"avg(servers.cpu)":                    wantFloat("4.66666666666666666666666666666666666666666666666666666666666666666666666666664"),
		"min(servers.cpu)":                    wantInt(2),
		"max(servers.cpu)":                    wantInt(8),
		"sort([3, 1, 2])":                     []any{wantInt(1), wantInt(2), wantInt(3)},
		"sortDesc([3, 1, 2])":                 []any{wantInt(3), wantInt(2), wantInt(1)},
		"sortByDesc(servers, \"cpu\").host":   []any{"c", "b", "a"},
		"keys(app)":                           []any{"defaultRegion", "name"},
		"values({\"b\": 2, \"a\": 1})":        []any{wantInt(1), wantInt(2)},
		"unique([1, 2, 1, 3])":                []any{wantInt(1), wantInt(2), wantInt(3)},
		"compact([0, 1, null, \"\", \"ok\"])": []any{wantInt(1), "ok"},
		"join(servers.host, \",\")":           "a,b,c",
		"default(app?.missing, \"fallback\")": "fallback",
	}

	for src, expected := range tests {
		t.Run(src, func(t *testing.T) {
			value, err := v.Eval(MustExpression(src))
			require.NoError(t, err)
			requireDecodedEqual(t, expected, value)
		})
	}
}

func TestExpressionMethodCallSyntax(t *testing.T) {
	v := expressionFixture(t)

	tests := map[string]any{
		"servers.cpu.sum()":                        wantInt(14),
		"servers?[enabled].cpu.avg()":              wantFloat("5"),
		"servers.sortByDesc(\"cpu\").host.first()": "c",
		"servers.host.contains(\"b\")":             true,
		"app?.missing.default(\"fallback\")":       "fallback",
		"servers?[enabled].sortBy(\"host\").host":  []any{"a", "c"},
		"servers?[enabled].cpu.sortDesc().first()": wantInt(8),
	}

	for src, expected := range tests {
		t.Run(src, func(t *testing.T) {
			value, err := v.Eval(MustExpression(src))
			require.NoError(t, err)
			requireDecodedEqual(t, expected, value)
		})
	}
}

func TestExpressionFunctionErrors(t *testing.T) {
	for _, src := range []string{
		"sum([1, \"bad\"])",
		"sort([1, \"bad\"])",
		"sortBy(servers, 1)",
		"join(servers.host, 1)",
		"contains(1, 1)",
	} {
		t.Run(src, func(t *testing.T) {
			_, err := ParseExpression(src)
			require.Error(t, err)
		})
	}
}

func TestExpressionAggregateEmptyArrays(t *testing.T) {
	v := expressionFixture(t)

	value, err := v.Eval(MustExpression(`sum([])`))
	require.NoError(t, err)
	require.Equal(t, wantInt(0), value)

	value, err = v.Eval(MustExpression(`avg([])`))
	require.NoError(t, err)
	require.Nil(t, value)

	value, err = v.Eval(MustExpression(`min([])`))
	require.NoError(t, err)
	require.Nil(t, value)

	value, err = v.Eval(MustExpression(`max([])`))
	require.NoError(t, err)
	require.Nil(t, value)
}

func TestExpressionCustomFunctions(t *testing.T) {
	expectedErr := errors.New("boom")
	v := New(
		WithGoFunction("wrap", func(s string) string { return "[" + s + "]" }),
		WithGoFunction("double", func(v int) int { return v * 2 }),
		WithGoFunction("prefix", func(s string, p string) string { return p + s }),
		WithGoFunction("fail", func() (any, error) { return nil, expectedErr }),
		WithGoFunction("failOnly", func() error { return expectedErr }),
		WithFunction("path", Function(func(ctx FunctionContext, _ []Value) (Value, error) {
			return String(ctx.Path.String()), nil
		})),
	)
	require.NoError(t, v.Set(MustPath("name"), "dragon"))

	value, err := v.EvalString(`wrap(name)`)
	require.NoError(t, err)
	require.Equal(t, "[dragon]", value)

	value, err = v.EvalString(`double(21)`)
	require.NoError(t, err)
	require.Equal(t, wantInt(42), value)

	value, err = v.EvalString(`name.prefix("hello ")`)
	require.NoError(t, err)
	require.Equal(t, "hello dragon", value)

	value, err = v.EvalString(`path()`)
	require.NoError(t, err)
	require.Equal(t, "", value)

	_, err = v.EvalString(`fail()`)
	require.ErrorIs(t, err, expectedErr)

	_, err = v.EvalString(`failOnly()`)
	require.ErrorIs(t, err, expectedErr)

	expr, err := v.ParseExpression(`wrap(name)`)
	require.NoError(t, err)
	value, err = v.Eval(expr)
	require.NoError(t, err)
	require.Equal(t, "[dragon]", value)

	_, err = v.ParseExpression(`missing(name)`)
	require.Error(t, err)

	_, err = v.CompileExpression(`wrap(1)`)
	require.Error(t, err)
}

func TestExpressionCustomValueFunctionAsMethod(t *testing.T) {
	v := New(WithFunction("surround", Function(func(_ FunctionContext, args []Value) (Value, error) {
		if len(args) != 3 {
			return Value{}, fmt.Errorf("surround expects 3 arguments, got %d", len(args))
		}
		value, _ := args[0].StringValue()
		left, _ := args[1].StringValue()
		right, _ := args[2].StringValue()
		return String(left + value + right), nil
	})))
	require.NoError(t, v.Set(MustPath("name"), "dragon"))

	value, err := v.EvalString(`name.surround("[", "]")`)
	require.NoError(t, err)
	require.Equal(t, "[dragon]", value)
}

func TestExpressionCustomValueFunctionMap(t *testing.T) {
	v := New(WithFunctions(map[string]Function{
		"label": func(ctx FunctionContext, args []Value) (Value, error) {
			if len(args) != 1 {
				return Value{}, fmt.Errorf("label expects 1 argument, got %d", len(args))
			}
			value, ok := args[0].StringValue()
			if !ok {
				return Value{}, fmt.Errorf("label expects string")
			}
			return String(ctx.Path.String() + ":" + value), nil
		},
	}))
	require.NoError(t, v.Set(MustPath("name"), "dragon"))

	value, err := v.EvalString(`label(name)`)
	require.NoError(t, err)
	require.Equal(t, ":dragon", value)
}

func TestExpressionMethodCallErrors(t *testing.T) {
	v := expressionFixture(t)

	_, err := ParseExpression(`app?.name()`)
	require.Error(t, err)

	_, err = v.CompileExpression(`app.name()`)
	require.Error(t, err)

	_, err = v.CompileExpression(`servers.cpu.sum("extra")`)
	require.Error(t, err)
}

func TestCustomFunctionCannotOverrideBuiltIn(t *testing.T) {
	require.Panics(t, func() {
		_ = New(WithGoFunction("sum", func() int { return 1 }))
	})
}

func TestExpressionCustomFunctionRejectsInvalidReturnSignature(t *testing.T) {
	v := New(WithGoFunction("bad", func() (string, string) {
		return "", "not error"
	}))

	_, err := v.CompileExpression(`bad()`)
	require.Error(t, err)
}
