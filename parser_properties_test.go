package variables

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPropertiesDefaultValuesAreStrings(t *testing.T) {
	props := `# comment
! another comment
app.name = demo
app.enabled=false
app.count: 12
app.empty
app.multiline=value \
  continued
app.escaped=hello\nworld
`

	v := New()
	require.NoError(t, v.Load(strings.NewReader(props), FormatProperties))

	tests := []struct {
		path string
		want any
	}{
		{path: "app.name", want: "demo"},
		{path: "app.enabled", want: "false"},
		{path: "app.count", want: "12"},
		{path: "app.empty", want: ""},
		{path: "app.multiline", want: "value continued"},
		{path: "app.escaped", want: "hello\nworld"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, ok := v.Get(MustPath(tt.path))
			require.True(t, ok)
			requireDecodedEqual(t, tt.want, got)
		})
	}
}

func TestPropertiesScalarInferenceIsExplicit(t *testing.T) {
	v := New()
	require.NoError(t, v.Load(strings.NewReader(`
app.enabled=true
app.count=12
app.float=1.25
app.nil=null
app.string=001
`), FormatProperties, WithScalarInference()))

	tests := []struct {
		path string
		want any
	}{
		{path: "app.enabled", want: true},
		{path: "app.count", want: wantInt(12)},
		{path: "app.float", want: wantFloat("1.25")},
		{path: "app.nil", want: nil},
		{path: "app.string", want: wantInt(1)},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, ok := v.Get(MustPath(tt.path))
			require.True(t, ok)
			requireDecodedEqual(t, tt.want, got)
		})
	}
}

func TestPropertiesSupportsEscapedSeparatorsInKeys(t *testing.T) {
	v := New()
	require.NoError(t, v.Load(strings.NewReader(`app["key\.with\.dot"] = value`), FormatProperties))

	value, ok := v.Get(MustPath(`app["key.with.dot"]`))
	require.True(t, ok)
	require.Equal(t, "value", value)
}

func TestPropertiesReportsInvalidPathAndTypeConflict(t *testing.T) {
	v := New()
	err := v.Load(strings.NewReader(`.=bad`), FormatProperties)
	require.ErrorIs(t, err, ErrInvalidPath)

	err = v.Load(strings.NewReader(`
app=leaf
app.name=demo
`), FormatProperties)
	require.ErrorIs(t, err, ErrTypeConflict)
}

func TestPropertiesReportsDanglingContinuation(t *testing.T) {
	v := New()
	err := v.Load(strings.NewReader(`app.name=value \`), FormatProperties)
	require.Error(t, err)
}

func TestPropertiesAcceptsLongValues(t *testing.T) {
	long := strings.Repeat("x", 128*1024)
	v := New()
	require.NoError(t, v.Load(strings.NewReader("app.long="+long), FormatProperties))

	value, ok := v.Get(MustPath("app.long"))
	require.True(t, ok)
	require.Equal(t, long, value)
}
