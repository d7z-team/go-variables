package variables

import (
	"strings"
	"testing"

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

	assert.NoError(t, err)
	s, err := variables.Template()("{{index .data 0 }}")
	assert.NoError(t, err)
	assert.Equal(t, "dragon", s)
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
