package variables

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func assertProp(t *testing.T, data, props string) {
	pr := NewVariables()
	assert.NoError(t, pr.FromProperties(props, ""))
	target, _ := json.Marshal(pr)
	assert.Equal(t, data, string(target))
}

func TestArrayProperties(t *testing.T) {
	assertProp(t, `{"foo":[{"bar":"bar1"},{"bar":"bar2"}]}`,
		`
foo.0.bar=bar1
foo.1.bar=bar2
`)
	assertProp(t, `{"foo":[null,{"bar":"bar1"},{"bar":"bar2"}]}`,
		`foo.1.bar=bar1
foo.-1.bar=bar2
`)
}

func TestArrayValue(t *testing.T) {
	assertProp(t, `{"foo":["bar1","bar2"]}`, `
foo.0=bar1
foo.1=bar2
`)

	assertProp(t, `{"foo":["","bar1","bar2"]}`, `
foo.1=bar1
foo.-1=bar2`)
	assertProp(t, `{"foo":["bar2","bar1"]}`, `
foo.1=bar1
foo.0=bar2
`)
}

func TestCompile(t *testing.T) {
	properties := NewVariables()
	assert.NoError(t, properties.FromProperties(`
a=b
b=12
c=true
d=121212.121212
e=null
f={{.d}}+{{.e}}
g.0=1
g.1={{index .g 0}}
g.2=1
foo=1
bar=2
aaa=true
bbb=false

`, ""))
	assert.NoError(t, properties.Compile())
	marshal, err := json.Marshal(properties)
	fmt.Println(string(marshal), err)
	_, err = properties.Execute("bbb")
	assert.NoError(t, err)
}
