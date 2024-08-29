package variables

import (
	"github.com/stretchr/testify/assert"
	"testing"
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
