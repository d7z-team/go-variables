package variables

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromJson(t *testing.T) {
	variables := NewVariables()
	err := variables.FromJson(`{
  "metadata": {
    "name": "dragon"
  },
  "data": [
    "aaa",
    "bbb"
  ]
}`, "")

	assert.NoError(t, err)
	assert.Equal(t, "dragon", variables.Get("metadata.name"))
	assert.Equal(t, "aaa", variables.Get("data.0"))
}

func TestVariables_FromJsonFilter(t *testing.T) {
	variables := NewVariables()
	err := variables.FromJsonFilter(`{
  "metadata": {
    "name": "dragon",
    "version": "1.0.0"
  },
  "other": "value"
}`, "", func(key string) bool {
		return strings.HasPrefix(key, "metadata.")
	})
	assert.NoError(t, err)
	assert.Equal(t, "dragon", variables.Get("metadata.name"))
	assert.Equal(t, "1.0.0", variables.Get("metadata.version"))
	assert.Nil(t, variables.Get("other"))
}
