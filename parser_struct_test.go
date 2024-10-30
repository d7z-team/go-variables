package variables

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnmarshal(t *testing.T) {
	type testCase struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	variables := NewVariables()
	assert.NoError(t, variables.FromYaml(`
metadata:
  name: dragon
  age: 12
`, ""))

	t2 := &testCase{}
	assert.NoError(t, variables.Unmarshal("metadata", t2))
	assert.Equal(t, &testCase{
		Name: "dragon",
		Age:  12,
	}, t2)
}
