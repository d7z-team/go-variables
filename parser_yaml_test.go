package variables

import (
	"fmt"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/stretchr/testify/assert"
)

func TestFromYaml(t *testing.T) {
	variables := NewVariables()
	err := variables.FromYaml(`
metadata:
  name: dragon
data:
 - "{{.metadata.name}}"
 - bbb
`, "")

	assert.NoError(t, err)
	s, err := variables.Template()("{{index .data 0 }}")
	assert.NoError(t, err)
	assert.Equal(t, "dragon", s)
}

func TestBigFromYaml(t *testing.T) {
	variables := NewVariables()
	err := variables.FromYaml(`
metadata:
  name: dragon
data:
 - "{{.metadata.name}}"
 - name: dragon
   value: test
 - bbb
`, "")
	marshal, err := yaml.Marshal(variables)
	fmt.Printf("%s\n", string(marshal))
	assert.NoError(t, err)
}

func TestVariables_FromYamlFilter(t *testing.T) {
	variables := NewVariables()
	err := variables.FromYamlFilter(`
metadata:
  name: dragon
  version: 1.0.0
`, "", func(key string) bool {
		return strings.HasPrefix(key, "metadata.")
	})
	assert.NoError(t, err)
	s, err := variables.Template()("{{.metadata.name }}-{{.metadata.version}}")
	assert.NoError(t, err)
	assert.Equal(t, "dragon-1.0.0", s)
}

func TestVariables_Get(t *testing.T) {
	variables := NewVariables()
	err := variables.FromYaml(`
metadata:
  name: dragon
data:
 - "{{.metadata.name}}"
 - bbb
 - ccc:
    name: "data"
`, "")
	assert.NoError(t, err)
	assert.Equal(t, variables.Get("metadata.name"), "dragon")
	assert.Equal(t, variables.Get("data.0"), "dragon")
	assert.Equal(t, variables.Get("data.2.ccc.name"), "data")
	assert.Equal(t, variables.Get("data.-1"), nil)
}
