package variables

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPropertiesContinuation(t *testing.T) {
	v := NewVariables()
	prop := `key=value \
    continued`
	// Expected: "value continued" or "value     continued"?
	// Java properties trims leading whitespace.

	err := v.FromProperties(prop, "")
	assert.NoError(t, err)

	val := v.Get("key")
	t.Logf("Got value: '%v'", val)
}
