package variables

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadArgsDefaultsToStrings(t *testing.T) {
	v := New()
	require.NoError(t, v.LoadArgs([]string{
		"app.name=demo",
		"app.enabled=true",
		`app["key.with.dot"]=value`,
	}))

	value, ok := v.Get(MustPath("app.enabled"))
	require.True(t, ok)
	require.Equal(t, "true", value)

	value, ok = v.Get(MustPath(`app["key.with.dot"]`))
	require.True(t, ok)
	require.Equal(t, "value", value)
}

func TestLoadArgsScalarInferencePrefixAndReplace(t *testing.T) {
	v := New()
	require.NoError(t, v.LoadArgs([]string{
		"enabled=true",
		"count=12",
	}, WithPrefix(MustPath("app")), WithScalarInference()))

	value, ok := v.Get(MustPath("app.enabled"))
	require.True(t, ok)
	require.Equal(t, true, value)

	value, ok = v.Get(MustPath("app.count"))
	require.True(t, ok)
	require.Equal(t, wantInt(12), value)
}

func TestLoadArgsReportsInvalidArgsPathAndTypeConflict(t *testing.T) {
	v := New()
	err := v.LoadArgs([]string{"missing-separator"})
	require.Error(t, err)

	err = v.LoadArgs([]string{"app..name=value"})
	require.ErrorIs(t, err, ErrInvalidPath)

	err = v.LoadArgs([]string{"app=leaf", "app.name=demo"})
	require.ErrorIs(t, err, ErrTypeConflict)
}
