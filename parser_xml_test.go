package variables

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadXMLMapsAttributesTextAndRepeatedElements(t *testing.T) {
	v := New()
	require.NoError(t, v.Load(strings.NewReader(`
<root>
  <item id="1">first</item>
  <item id="2">second</item>
  <single enabled="true">
    <name>demo</name>
  </single>
</root>`), FormatXML))

	tests := []struct {
		path string
		want any
	}{
		{path: `root.item[0].@id`, want: "1"},
		{path: `root.item[0].#text`, want: "first"},
		{path: `root.item[1].@id`, want: "2"},
		{path: `root.item[1].#text`, want: "second"},
		{path: `root.single.@enabled`, want: "true"},
		{path: `root.single.name`, want: "demo"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, ok := v.Get(MustPath(tt.path))
			require.True(t, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestLoadXMLWithPrefixAndInvalidXML(t *testing.T) {
	v := New()
	require.NoError(t, v.Load(strings.NewReader(`<root><name>demo</name></root>`), FormatXML, WithPrefix(MustPath("doc"))))
	value, ok := v.Get(MustPath("doc.root.name"))
	require.True(t, ok)
	require.Equal(t, "demo", value)

	err := v.Load(strings.NewReader(`<root>`), FormatXML)
	require.Error(t, err)
}
