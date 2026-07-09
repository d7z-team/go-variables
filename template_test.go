package variables

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderUsesSnapshotAndCustomFunctions(t *testing.T) {
	v := New(WithGoFunction("upper", strings.ToUpper), WithGoFunctions(map[string]any{
		"wrap": func(s string) string { return "[" + s + "]" },
	}))
	require.NoError(t, v.Set(MustPath("name"), "dragon"))

	rendered, err := v.Render(`{{wrap (upper .name)}}`)
	require.NoError(t, err)
	require.Equal(t, "[DRAGON]", rendered)
}

func TestRenderReportsParseAndMissingKeyErrors(t *testing.T) {
	v := New()
	_, err := v.Render(`{{`)
	require.Error(t, err)

	_, err = v.Render(`{{.missing}}`)
	require.Error(t, err)
}

func TestInterpolateUsesCompleteSnapshot(t *testing.T) {
	v := New()
	require.NoError(t, v.Load(strings.NewReader(`
greeting: "hello {{.name}}"
name: dragon
`), FormatYAML))
	require.NoError(t, v.Interpolate())

	value, ok := v.Get(MustPath("greeting"))
	require.True(t, ok)
	require.Equal(t, "hello dragon", value)
}

func TestInterpolateNonRecursiveLeavesSecondPassTemplates(t *testing.T) {
	v := New()
	require.NoError(t, v.Load(strings.NewReader(`
a: "{{.b}}"
b: "{{.c}}"
c: ok
`), FormatYAML))
	require.NoError(t, v.Interpolate())

	value, ok := v.Get(MustPath("a"))
	require.True(t, ok)
	require.Equal(t, "{{.c}}", value)
}

func TestInterpolateRecursiveResolvesNestedTemplatesAndDetectsCycles(t *testing.T) {
	v := New()
	require.NoError(t, v.Load(strings.NewReader(`
a: "{{.b}}"
b: "{{.c}}"
c: ok
`), FormatYAML))
	require.NoError(t, v.Interpolate(WithRecursiveInterpolation()))
	value, ok := v.Get(MustPath("a"))
	require.True(t, ok)
	require.Equal(t, "ok", value)

	cyclic := New()
	require.NoError(t, cyclic.Load(strings.NewReader(`a: "{{.a}}"`), FormatYAML))
	require.ErrorIs(t, cyclic.Interpolate(WithRecursiveInterpolation()), ErrCycleDetected)
}

func TestInterpolateHonorsMaxDepth(t *testing.T) {
	v := New()
	require.NoError(t, v.Load(strings.NewReader(`
a: "{{.b}}"
b: "{{.c}}"
c: ok
`), FormatYAML))
	require.ErrorIs(t, v.Interpolate(WithRecursiveInterpolation(), WithMaxDepth(1)), ErrCycleDetected)
}

func TestInterpolateReportsTemplateAndExpressionErrors(t *testing.T) {
	missing := New()
	require.NoError(t, missing.Load(strings.NewReader(`a: "{{.missing}}"`), FormatYAML))
	require.Error(t, missing.Interpolate())

	badExpr := New()
	require.NoError(t, badExpr.Load(strings.NewReader(`a: "${{ missing + }}"`), FormatYAML))
	require.Error(t, badExpr.Interpolate())

	badFunction := New()
	require.NoError(t, badFunction.Load(strings.NewReader(`a: "${{ missing(1) }}"`), FormatYAML))
	require.Error(t, badFunction.Interpolate())
}

func TestExpressionInterpolationReturnsTypedValues(t *testing.T) {
	v := New()
	require.NoError(t, v.Load(strings.NewReader(`
app:
  port: 8080
servers:
  - host: a
    enabled: true
    cpu: 2
  - host: b
    enabled: false
    cpu: 4
enabledHosts: "${{ servers?[enabled].host }}"
totalCPU: "${{ sum(servers.cpu) }}"
message: "port=${{ app.port }}"
`), FormatYAML))
	require.NoError(t, v.Interpolate())

	value, ok := v.Get(MustPath("enabledHosts"))
	require.True(t, ok)
	require.Equal(t, []any{"a"}, value)

	value, ok = v.Get(MustPath("totalCPU"))
	require.True(t, ok)
	require.Equal(t, wantInt(6), value)

	value, ok = v.Get(MustPath("message"))
	require.True(t, ok)
	require.Equal(t, "port=8080", value)
}

func TestExpressionInterpolationUsesCustomFunctions(t *testing.T) {
	v := New(WithGoFunction("wrap", func(s string) string { return "[" + s + "]" }))
	require.NoError(t, v.Load(strings.NewReader(`
name: dragon
wrapped: "${{ wrap(name) }}"
`), FormatYAML))
	require.NoError(t, v.Interpolate())

	value, ok := v.Get(MustPath("wrapped"))
	require.True(t, ok)
	require.Equal(t, "[dragon]", value)
}

func TestExpressionInterpolationHandlesPlaceholderBoundaries(t *testing.T) {
	v := New()
	require.NoError(t, v.Load(strings.NewReader(`
literal: 'value=${{ "}}" }}'
`), FormatYAML))
	require.NoError(t, v.Interpolate())

	value, ok := v.Get(MustPath("literal"))
	require.True(t, ok)
	require.Equal(t, "value=}}", value)

	singleQuoted := New()
	require.NoError(t, singleQuoted.Load(strings.NewReader(`
literal: "value=${{ '}}' }}"
`), FormatYAML))
	require.NoError(t, singleQuoted.Interpolate())

	value, ok = singleQuoted.Get(MustPath("literal"))
	require.True(t, ok)
	require.Equal(t, "value=}}", value)

	empty := New()
	require.NoError(t, empty.Load(strings.NewReader(`bad: "${{ }}"`), FormatYAML))
	require.Error(t, empty.Interpolate())
}
