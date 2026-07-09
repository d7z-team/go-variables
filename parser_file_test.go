package variables

import (
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadJSONPreservesTypesAndUsesJSONNumber(t *testing.T) {
	v := New()
	require.NoError(t, v.Load(strings.NewReader(`{
		"quotedBool": "false",
		"quotedNumber": "001",
		"number": 12,
		"float": 1.25,
		"flag": false,
		"nil": null,
		"array": [1, "2"]
	}`), FormatJSON))

	tests := []struct {
		path string
		want any
	}{
		{path: "quotedBool", want: "false"},
		{path: "quotedNumber", want: "001"},
		{path: "number", want: wantInt(12)},
		{path: "float", want: wantFloat("1.25")},
		{path: "flag", want: false},
		{path: "nil", want: nil},
		{path: "array[0]", want: wantInt(1)},
		{path: "array[1]", want: "2"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, ok := v.Get(MustPath(tt.path))
			require.True(t, ok)
			requireDecodedEqual(t, tt.want, got)
		})
	}
}

func TestLoadJSONPreservesHugeNumbers(t *testing.T) {
	v := New()
	require.NoError(t, v.Load(strings.NewReader(`{
		"big": 123456789012345678901234567890,
		"decimal": 1.234567890123456789
	}`), FormatJSON))

	value, ok := v.Get(MustPath("big"))
	require.True(t, ok)
	require.Equal(t, "123456789012345678901234567890", value.(*big.Int).String())

	value, ok = v.Get(MustPath("decimal"))
	require.True(t, ok)
	requireBigFloatEqual(t, "1.234567890123456789", value)
}

func TestLoadString(t *testing.T) {
	v := New()
	require.NoError(t, v.LoadString(`{"app":{"name":"demo"}}`, FormatJSON))

	value, ok := v.Get(MustPath("app.name"))
	require.True(t, ok)
	require.Equal(t, "demo", value)
}

func TestLoadJSONDoesNotInferScalarsWhenOptionIsProvided(t *testing.T) {
	v := New()
	require.NoError(t, v.Load(strings.NewReader(`{"quotedBool":"false","quotedNull":"null"}`), FormatJSON, WithScalarInference()))

	value, ok := v.Get(MustPath("quotedBool"))
	require.True(t, ok)
	require.Equal(t, "false", value)
	value, ok = v.Get(MustPath("quotedNull"))
	require.True(t, ok)
	require.Equal(t, "null", value)
}

func TestLoadYAMLPreservesQuotedStringsAndNativeScalars(t *testing.T) {
	v := New()
	require.NoError(t, v.Load(strings.NewReader(`
quotedBool: "false"
quotedNumber: "001"
flag: false
integer: 12
float: 1.5
nil: null
items:
  - "1"
  - 2
`), FormatYAML))

	tests := []struct {
		path string
		want any
	}{
		{path: "quotedBool", want: "false"},
		{path: "quotedNumber", want: "001"},
		{path: "flag", want: false},
		{path: "integer", want: wantInt(12)},
		{path: "float", want: wantFloat("1.5")},
		{path: "nil", want: nil},
		{path: "items[0]", want: "1"},
		{path: "items[1]", want: wantInt(2)},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, ok := v.Get(MustPath(tt.path))
			require.True(t, ok)
			requireDecodedEqual(t, tt.want, got)
		})
	}
}

func TestLoadRejectsInvalidFormatAndInvalidInput(t *testing.T) {
	v := New()
	err := v.Load(strings.NewReader(`{`), FormatJSON)
	require.Error(t, err)

	err = v.Load(strings.NewReader(`{"a":1} {"b":2}`), FormatJSON)
	require.Error(t, err)

	err = v.Load(strings.NewReader(`x`), Format("toml"))
	require.ErrorIs(t, err, ErrUnsupportedFormat)
}

func TestFormatFromFile(t *testing.T) {
	tests := map[string]Format{
		"config.JSON":       FormatJSON,
		"config.yaml":       FormatYAML,
		"config.yml":        FormatYAML,
		"config.xml":        FormatXML,
		"config.properties": FormatProperties,
		"config.prop":       FormatProperties,
	}

	for file, want := range tests {
		t.Run(file, func(t *testing.T) {
			got, err := FormatFromFile(file)
			require.NoError(t, err)
			require.Equal(t, want, got)
		})
	}

	_, err := FormatFromFile("config.env")
	require.ErrorIs(t, err, ErrUnsupportedFormat)
}

func TestLoadFileWithPrefixAndFilter(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(file, []byte(`{"allowed":1,"denied":2}`), 0o600))

	v := New()
	require.NoError(t, v.LoadFile(file,
		WithPrefix(MustPath("cfg")),
		WithFilter(func(path Path, _ any) bool {
			return path.IsRoot() || path.String() == "cfg" || path.String() == "cfg.allowed"
		}),
	))

	value, ok := v.Get(MustPath("cfg.allowed"))
	require.True(t, ok)
	require.Equal(t, wantInt(1), value)
	_, ok = v.Get(MustPath("cfg.denied"))
	require.False(t, ok)
}

func TestLoadFileReportsOpenAndUnsupportedErrors(t *testing.T) {
	v := New()
	err := v.LoadFile(filepath.Join(t.TempDir(), "missing.json"))
	require.Error(t, err)

	file := filepath.Join(t.TempDir(), "config.env")
	require.NoError(t, os.WriteFile(file, []byte("x=y"), 0o600))
	err = v.LoadFile(file)
	require.ErrorIs(t, err, ErrUnsupportedFormat)
}

func TestMergeModes(t *testing.T) {
	v := New()
	require.NoError(t, v.Load(strings.NewReader(`{"app":{"name":"demo","ports":[80],"nested":{"a":1}}}`), FormatJSON))
	require.NoError(t, v.Load(strings.NewReader(`{"app":{"version":"next","ports":[443],"nested":{"b":2}}}`), FormatJSON, WithMergeMode(DeepMerge)))

	var app struct {
		Name    string         `json:"name"`
		Version string         `json:"version"`
		Ports   []int          `json:"ports"`
		Nested  map[string]int `json:"nested"`
	}
	require.NoError(t, v.Decode(MustPath("app"), &app))
	require.Equal(t, "demo", app.Name)
	require.Equal(t, "next", app.Version)
	require.Equal(t, []int{443}, app.Ports)
	require.Equal(t, map[string]int{"a": 1, "b": 2}, app.Nested)

	require.NoError(t, v.Load(strings.NewReader(`{"app":{"name":"replaced"}}`), FormatJSON, WithMergeMode(Replace)))
	value, ok := v.Get(MustPath("app.version"))
	require.False(t, ok)
	require.Nil(t, value)

	err := v.Load(strings.NewReader(`{"x":1}`), FormatJSON, WithPrefix(MustPath("app")), WithMergeMode(ErrorOnConflict))
	require.ErrorIs(t, err, ErrTypeConflict)
}

func TestErrorOnConflictAtRoot(t *testing.T) {
	v := New()
	require.NoError(t, v.Load(strings.NewReader(`{"x":1}`), FormatJSON))
	err := v.Load(strings.NewReader(`{"y":2}`), FormatJSON, WithMergeMode(ErrorOnConflict))
	require.ErrorIs(t, err, ErrTypeConflict)

	scalar := New()
	require.NoError(t, scalar.Set(Root(), "value"))
	err = scalar.Load(strings.NewReader(`{"y":2}`), FormatJSON, WithMergeMode(ErrorOnConflict))
	require.ErrorIs(t, err, ErrTypeConflict)

	empty := New()
	require.NoError(t, empty.Set(Root(), []any{}))
	require.NoError(t, empty.Load(strings.NewReader(`{"y":2}`), FormatJSON, WithMergeMode(ErrorOnConflict)))
}

func TestFilterCanDropRootOrArrayElements(t *testing.T) {
	dropped := New()
	require.NoError(t, dropped.Load(strings.NewReader(`{"x":1}`), FormatJSON, WithFilter(func(Path, any) bool {
		return false
	})))
	value, ok := dropped.Get(Root())
	require.True(t, ok)
	require.Equal(t, map[string]any{}, value)

	filtered := New()
	require.NoError(t, filtered.Load(strings.NewReader(`{"items":["a","b","c"]}`), FormatJSON, WithFilter(func(path Path, _ any) bool {
		return path.String() != "items[1]"
	})))
	value, ok = filtered.Get(MustPath("items"))
	require.True(t, ok)
	require.Equal(t, []any{"a", "c"}, value)
}

func TestDecodeReportsMissingAndDecodeErrors(t *testing.T) {
	v := New()
	err := v.Decode(MustPath("missing"), &struct{}{})
	require.ErrorIs(t, err, ErrNotFound)

	require.NoError(t, v.Set(MustPath("app.name"), "demo"))
	var dst struct {
		Name int `json:"name"`
	}
	err = v.Decode(MustPath("app"), &dst)
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrNotFound))
}
