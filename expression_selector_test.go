package variables

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpressionSelectsPathsAndProjectsArrays(t *testing.T) {
	v := expressionFixture(t)

	matches, err := v.Select(MustExpression("servers.host"))
	require.NoError(t, err)
	require.Equal(t, []Match{
		{Path: MustPath("servers[0].host"), Value: String("a")},
		{Path: MustPath("servers[1].host"), Value: String("b")},
		{Path: MustPath("servers[2].host"), Value: String("c")},
	}, matches)

	values, err := v.SelectValues(MustExpression("servers?[enabled].host"))
	require.NoError(t, err)
	require.Equal(t, []any{"a", "c"}, values)

	first, ok, err := v.First(MustExpression("servers?[cpu >= 4].host"))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, Match{Path: MustPath("servers[1].host"), Value: String("b")}, first)

	count, err := v.Count(MustExpression("servers?[enabled]"))
	require.NoError(t, err)
	require.Equal(t, 2, count)
}

func TestExpressionFiltersSupportRootAndMissingFields(t *testing.T) {
	v := expressionFixture(t)

	matches, err := v.Select(MustExpression(`servers?[region == $.app.defaultRegion].host`))
	require.NoError(t, err)
	require.Equal(t, []Match{
		{Path: MustPath("servers[0].host"), Value: String("a")},
		{Path: MustPath("servers[2].host"), Value: String("c")},
	}, matches)

	matches, err = v.Select(MustExpression(`servers?[missing].host`))
	require.NoError(t, err)
	require.Empty(t, matches)
}

func TestExpressionOptionalAccess(t *testing.T) {
	v := expressionFixture(t)

	value, err := v.Eval(MustExpression("app?.missing"))
	require.NoError(t, err)
	require.Nil(t, value)

	value, err = v.Eval(MustExpression("servers?.missing"))
	require.NoError(t, err)
	require.Equal(t, []any{nil, nil, nil}, value)

	_, err = v.Eval(MustExpression("app.missing"))
	require.Error(t, err)
}

func TestExpressionSelectReturnsCopies(t *testing.T) {
	v := expressionFixture(t)

	matches, err := v.Select(MustExpression("servers"))
	require.NoError(t, err)
	require.Len(t, matches, 1)
	matches[0].Value.arr[0].obj["host"] = String("changed")

	value, ok := v.Get(MustPath("servers[0].host"))
	require.True(t, ok)
	require.Equal(t, "a", value)
}

func TestExpressionSelectRejectsComputedValues(t *testing.T) {
	v := expressionFixture(t)
	_, err := v.Select(MustExpression("sum(servers.cpu)"))
	require.Error(t, err)
}

func expressionFixture(t *testing.T) *Variables {
	t.Helper()

	v := New()
	require.NoError(t, v.Set(Root(), map[string]any{
		"app": map[string]any{
			"name":          "demo",
			"defaultRegion": "us",
		},
		"servers": []any{
			map[string]any{"host": "a", "enabled": true, "cpu": 2, "region": "us", "tags": []any{"stable", "blue"}},
			map[string]any{"host": "b", "enabled": false, "cpu": 4, "region": "eu", "tags": []any{"canary"}},
			map[string]any{"host": "c", "enabled": true, "cpu": 8, "region": "us", "tags": []any{"stable"}},
		},
	}))
	return v
}
