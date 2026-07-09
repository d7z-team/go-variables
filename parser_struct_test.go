package variables

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadStructSupportsPointersPrefixAndDecode(t *testing.T) {
	type config struct {
		Name string `json:"name"`
		Port int    `json:"port"`
	}

	v := New()
	require.NoError(t, v.LoadStruct(&config{Name: "demo", Port: 8080}, WithPrefix(MustPath("app"))))

	var out config
	require.NoError(t, v.Decode(MustPath("app"), &out))
	require.Equal(t, config{Name: "demo", Port: 8080}, out)
}

func TestLoadStructUsesJSONTagsAndCanFilter(t *testing.T) {
	type config struct {
		Public string `json:"public"`
		Secret string `json:"secret"`
	}

	v := New()
	require.NoError(t, v.LoadStruct(config{Public: "ok", Secret: "hidden"}, WithFilter(func(path Path, _ any) bool {
		return path.IsRoot() || path.String() != "secret"
	})))

	value, ok := v.Get(MustPath("public"))
	require.True(t, ok)
	require.Equal(t, "ok", value)
	_, ok = v.Get(MustPath("secret"))
	require.False(t, ok)
}

func TestLoadStructReportsMarshalErrors(t *testing.T) {
	v := New()
	err := v.LoadStruct(map[string]any{"bad": func() {}})
	require.Error(t, err)
}

func TestDecodeJSONNumbersIntoTypedStruct(t *testing.T) {
	v := New()
	require.NoError(t, v.LoadStruct(map[string]any{"n": json.Number("12")}))

	var out struct {
		N int `json:"n"`
	}
	require.NoError(t, v.Decode(Root(), &out))
	require.Equal(t, 12, out.N)
}
