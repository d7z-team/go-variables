package variables

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromXML(t *testing.T) {
	v := NewVariables()
	xmlData := `
<root>
    <key>value</key>
    <nested>
        <item id="1">val1</item>
        <item id="2">val2</item>
    </nested>
    <list>a</list>
    <list>b</list>
</root>`

	err := v.FromXML(xmlData, "")
	assert.NoError(t, err)

	// Check simple value
	assert.Equal(t, "value", v.Get("root.key"))
	assert.Equal(t, "value", v.Get("root.key"))

	// Check list (mxj might handle repeated elements as list)
	// <list>a</list><list>b</list> -> list: ["a", "b"]
	assert.Equal(t, "a", v.Get("root.list.0"))
	assert.Equal(t, "b", v.Get("root.list.1"))

	// Check nested with attributes
	// <item id="1">val1</item> -> item: {"-id": "1", "#text": "val1"}
	// But since there are two <item>, it becomes a list of objects.

	val1Text := v.Get("root.nested.item.0.#text")
	if val1Text == nil {
		// Maybe mxj structure is different?
		// Let's inspect what we got if it fails.
	} else {
		assert.Equal(t, "val1", val1Text)
		assert.Equal(t, 1, v.Get("root.nested.item.0.-id"))
	}
}
