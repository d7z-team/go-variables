package variables

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNestedArray(t *testing.T) {
	v := NewVariables()
	// array[0] is an array, array[0][0] = "val"
	// Key: "array.0.0" -> value: "val"

	// We need to initialize the structure first or rely on Set to create it.
	// Set("array.0.0", "val")

	err := v.Set("array.0.0", "val")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val := v.Get("array.0.0")
	assert.Equal(t, "val", val)

	arr := v.Get("array")
	// Verify structure
	if a, ok := arr.([]any); ok {
		if len(a) > 0 {
			if sub, ok := a[0].([]any); ok {
				if len(sub) > 0 {
					assert.Equal(t, "val", sub[0])
				} else {
					t.Error("subarray empty")
				}
			} else {
				t.Errorf("expected []any, got %T", a[0])
			}
		} else {
			t.Error("array empty")
		}
	} else {
		t.Errorf("expected []any, got %T", arr)
	}
}
