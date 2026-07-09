package variables

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePathRootBareKeysIndexesAndQuotedKeys(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want Path
	}{
		{name: "root", src: "", want: Root()},
		{name: "bare keys", src: "app.server.host", want: Path{Key("app"), Key("server"), Key("host")}},
		{name: "root index", src: "[0].name", want: Path{Index(0), Key("name")}},
		{name: "index", src: "servers[10].host", want: Path{Key("servers"), Index(10), Key("host")}},
		{name: "quoted key", src: `app["key.with.dot"].value`, want: Path{Key("app"), Key("key.with.dot"), Key("value")}},
		{name: "single quoted key", src: `app[' spaced key '][0]`, want: Path{Key("app"), Key(" spaced key "), Index(0)}},
		{name: "quoted empty key", src: `app[""]`, want: Path{Key("app"), Key("")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePath(tt.src)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParsePathRejectsAmbiguousOrInvalidPaths(t *testing.T) {
	tests := []string{
		".app",
		"app.",
		"app..name",
		"app[]",
		"app[-1]",
		"app[abc]",
		"app[",
		"app.[0]",
		`app.["key"]`,
		`app["unterminated]`,
		`app["key"`,
		`app name[0]tail`,
	}

	for _, src := range tests {
		t.Run(src, func(t *testing.T) {
			_, err := ParsePath(src)
			require.ErrorIs(t, err, ErrInvalidPath)
		})
	}
}

func TestPathStringRoundTrip(t *testing.T) {
	paths := []Path{
		Root(),
		Path{Key("app"), Key("name")},
		Path{Key("app"), Key("key.with.dot"), Index(0), Key("@id")},
		Path{Key("app"), Key(""), Key("quote\"key")},
		Path{Key("app"), Key("space key"), Key("line\nbreak"), Key(`slash\key`)},
	}

	for _, path := range paths {
		t.Run(path.String(), func(t *testing.T) {
			roundTrip, err := ParsePath(path.String())
			require.NoError(t, err)
			require.Equal(t, path, roundTrip)
		})
	}
}

func TestPathStringQuotesSpecialKeys(t *testing.T) {
	path := Path{Key("app"), Key("space key"), Key("line\nbreak"), Key(`slash\key`)}
	require.Equal(t, `app["space key"]["line\nbreak"]["slash\\key"]`, path.String())

	roundTrip, err := ParsePath(path.String())
	require.NoError(t, err)
	require.Equal(t, path, roundTrip)
}

func TestJoinPathChildParentAndSegments(t *testing.T) {
	base := MustPath("app")
	joined := JoinPath(base, Key("servers"), Index(0))
	require.Equal(t, MustPath("app.servers[0]"), joined)

	child := base.Child(Key("name"))
	require.Equal(t, MustPath("app.name"), child)

	parent, leaf, ok := joined.Parent()
	require.True(t, ok)
	require.Equal(t, MustPath("app.servers"), parent)
	require.Equal(t, Index(0), leaf)

	_, _, ok = Root().Parent()
	require.False(t, ok)

	segments := joined.Segments()
	segments[0] = Key("changed")
	require.Equal(t, "app.servers[0]", joined.String())
}

func TestMustPathPanicsOnInvalidPath(t *testing.T) {
	require.Panics(t, func() {
		_ = MustPath("app..name")
	})
}

func TestPathErrorSupportsErrorsAsAndIs(t *testing.T) {
	err := (&PathError{Op: "set", Path: MustPath("app.name"), Err: ErrTypeConflict})

	var pathErr *PathError
	require.True(t, errors.As(err, &pathErr))
	require.ErrorIs(t, err, ErrTypeConflict)
	require.Equal(t, "set app.name: type conflict", err.Error())
}
