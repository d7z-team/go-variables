package variables

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetGetRootNestedMapsAndArrays(t *testing.T) {
	v := New()
	require.NoError(t, v.Set(Root(), map[string]any{"app": map[string]any{"name": "demo"}}))
	require.NoError(t, v.Set(MustPath(`app["key.with.dot"][0].host`), "localhost"))
	require.NoError(t, v.Set(MustPath("root.0"), "numeric-key"))
	require.NoError(t, v.Set(MustPath("root.items[2]"), "third"))

	value, ok := v.Get(MustPath("app.name"))
	require.True(t, ok)
	require.Equal(t, "demo", value)

	value, ok = v.Get(MustPath(`app["key.with.dot"][0].host`))
	require.True(t, ok)
	require.Equal(t, "localhost", value)

	value, ok = v.Get(MustPath("root.0"))
	require.True(t, ok)
	require.Equal(t, "numeric-key", value)

	items, ok := v.Get(MustPath("root.items"))
	require.True(t, ok)
	require.Equal(t, []any{nil, nil, "third"}, items)
}

func TestSetRejectsTypeConflictsAndInvalidIndexes(t *testing.T) {
	v := New()
	require.NoError(t, v.Set(MustPath("app"), "leaf"))

	err := v.Set(MustPath("app.name"), "demo")
	require.ErrorIs(t, err, ErrTypeConflict)

	err = v.Set(Path{Key("items"), Index(-1)}, "bad")
	require.ErrorIs(t, err, ErrIndexOutOfRange)

	var pathErr *PathError
	require.True(t, errors.As(err, &pathErr))
	require.Equal(t, Path{Key("items"), Index(-1)}, pathErr.Path)
}

func TestGetMissingAndWrongContainerReturnFalse(t *testing.T) {
	v := New()
	require.NoError(t, v.Set(MustPath("app.name"), "demo"))

	_, ok := v.Get(MustPath("app.missing"))
	require.False(t, ok)

	_, ok = v.Get(MustPath("app.name[0]"))
	require.False(t, ok)
}

func TestSnapshotAndGetDoNotExposeInternals(t *testing.T) {
	v := New()
	require.NoError(t, v.Set(MustPath("app"), map[string]any{"name": "demo", "items": []any{"a"}}))

	snapshot := v.Snapshot().(map[string]any)
	snapshot["app"].(map[string]any)["name"] = "changed"
	snapshot["app"].(map[string]any)["items"].([]any)[0] = "changed"

	value, ok := v.Get(MustPath("app.name"))
	require.True(t, ok)
	require.Equal(t, "demo", value)

	app, ok := v.Get(MustPath("app"))
	require.True(t, ok)
	app.(map[string]any)["name"] = "changed-again"
	app.(map[string]any)["items"].([]any)[0] = "changed-again"

	value, ok = v.Get(MustPath("app.name"))
	require.True(t, ok)
	require.Equal(t, "demo", value)
	value, ok = v.Get(MustPath("app.items[0]"))
	require.True(t, ok)
	require.Equal(t, "a", value)
}

func TestCloneCopiesDataAndOptions(t *testing.T) {
	v := New(WithGoFunction("mark", func(s string) string { return "base:" + s }))
	require.NoError(t, v.Set(MustPath("app.name"), "demo"))

	cloned := v.Clone()
	require.NoError(t, cloned.Set(MustPath("app.name"), "clone"))

	value, ok := v.Get(MustPath("app.name"))
	require.True(t, ok)
	require.Equal(t, "demo", value)
	value, ok = cloned.Get(MustPath("app.name"))
	require.True(t, ok)
	require.Equal(t, "clone", value)

	rendered, err := cloned.Render(`{{mark .app.name}}`)
	require.NoError(t, err)
	require.Equal(t, "base:clone", rendered)
}

func TestCloneCanOverrideOptions(t *testing.T) {
	v := New(WithGoFunction("mark", func(s string) string { return "base:" + s }))
	require.NoError(t, v.Set(MustPath("name"), "demo"))

	cloned := v.Clone(WithGoFunction("mark", func(s string) string { return "clone:" + s }))

	rendered, err := v.Render(`{{mark .name}}`)
	require.NoError(t, err)
	require.Equal(t, "base:demo", rendered)

	rendered, err = cloned.Render(`{{mark .name}}`)
	require.NoError(t, err)
	require.Equal(t, "clone:demo", rendered)
}

func TestSetClonesTypedMapsAndSlices(t *testing.T) {
	v := New()
	sourceMap := map[string]string{"name": "demo"}
	sourceSlice := []string{"a", "b"}

	require.NoError(t, v.Set(MustPath("typed.map"), sourceMap))
	require.NoError(t, v.Set(MustPath("typed.slice"), sourceSlice))

	sourceMap["name"] = "changed"
	sourceSlice[0] = "changed"

	value, ok := v.Get(MustPath("typed.map"))
	require.True(t, ok)
	gotMap := value.(map[string]any)
	gotMap["name"] = "changed-again"

	value, ok = v.Get(MustPath("typed.map"))
	require.True(t, ok)
	require.Equal(t, map[string]any{"name": "demo"}, value)

	value, ok = v.Get(MustPath("typed.slice"))
	require.True(t, ok)
	gotSlice := value.([]any)
	gotSlice[0] = "changed-again"

	value, ok = v.Get(MustPath("typed.slice"))
	require.True(t, ok)
	require.Equal(t, []any{"a", "b"}, value)
}

func TestAppendSupportsRootAndNestedSlices(t *testing.T) {
	v := New()
	require.NoError(t, v.Set(Root(), []any{"a"}))
	require.NoError(t, v.Append(Root(), "b"))

	value, ok := v.Get(Root())
	require.True(t, ok)
	require.Equal(t, []any{"a", "b"}, value)

	nested := New()
	require.NoError(t, nested.Set(MustPath("items"), []any{}))
	require.NoError(t, nested.Append(MustPath("items"), "first"))
	value, ok = nested.Get(MustPath("items[0]"))
	require.True(t, ok)
	require.Equal(t, "first", value)
}

func TestAppendReportsNotFoundAndTypeConflict(t *testing.T) {
	v := New()
	err := v.Append(MustPath("missing"), "value")
	require.ErrorIs(t, err, ErrNotFound)

	require.NoError(t, v.Set(MustPath("app"), "leaf"))
	err = v.Append(MustPath("app"), "value")
	require.ErrorIs(t, err, ErrTypeConflict)
}

func TestDeleteRootMapKeysAndSliceIndexes(t *testing.T) {
	v := New()
	require.NoError(t, v.Set(MustPath("app.name"), "demo"))
	require.NoError(t, v.Delete(MustPath("app.name")))
	_, ok := v.Get(MustPath("app.name"))
	require.False(t, ok)

	require.NoError(t, v.Set(MustPath("items"), []any{"a", "b"}))
	require.NoError(t, v.Delete(MustPath("items[0]")))
	value, ok := v.Get(MustPath("items[0]"))
	require.True(t, ok)
	require.Nil(t, value)

	require.NoError(t, v.Delete(Root()))
	value, ok = v.Get(Root())
	require.True(t, ok)
	require.Equal(t, map[string]any{}, value)
}

func TestDeleteReportsMissingTypeConflictAndOutOfRange(t *testing.T) {
	v := New()
	err := v.Delete(MustPath("missing"))
	require.ErrorIs(t, err, ErrNotFound)

	require.NoError(t, v.Set(MustPath("app"), "leaf"))
	err = v.Delete(MustPath("app.name"))
	require.ErrorIs(t, err, ErrTypeConflict)

	require.NoError(t, v.Set(MustPath("items"), []any{"a"}))
	err = v.Delete(MustPath("items[2]"))
	require.ErrorIs(t, err, ErrIndexOutOfRange)
}

func TestConcurrentSetGet(t *testing.T) {
	v := New()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			require.NoError(t, v.Set(Path{Key("items"), Index(i)}, i))
			value, _ := v.Get(Path{Key("items"), Index(i)})
			requireDecodedEqual(t, wantInt(int64(i)), value)
		}(i)
	}
	wg.Wait()

	value, ok := v.Get(Path{Key("items"), Index(99)})
	require.True(t, ok)
	requireDecodedEqual(t, wantInt(99), value)
}
