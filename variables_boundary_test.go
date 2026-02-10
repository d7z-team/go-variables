package variables

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVariables_BoundarySet(t *testing.T) {
	v := NewVariables()

	t.Run("EmptyKey", func(t *testing.T) {
		err := v.Set("", "value")
		assert.NoError(t, err)
		// With map implementation, Get("") returns the map itself if it hits the base case.
		res := v.Get("")
		assert.NotNil(t, res)
	})

	t.Run("DeeplyNested", func(t *testing.T) {
		err := v.Set("a.b.c.d.e.f.g", "val")
		assert.NoError(t, err)
		assert.Equal(t, "val", v.Get("a.b.c.d.e.f.g"))
	})

	t.Run("TypeConflict_MapToLeaf", func(t *testing.T) {
		v := NewVariables()
		assert.NoError(t, v.Set("a", "leaf"))
		// a is now a string. Trying to set a.b should fail.
		err := v.Set("a.b", "val")
		assert.Error(t, err)
	})

	t.Run("TypeConflict_ArrayToLeaf", func(t *testing.T) {
		v := NewVariables()
		assert.NoError(t, v.Set("a", "leaf"))
		// a is now a string. Trying to set a.0 should fail.
		err := v.Set("a.0", "val")
		assert.Error(t, err)
	})

	t.Run("TypeConflict_MapToArray", func(t *testing.T) {
		v := NewVariables()
		assert.NoError(t, v.Set("a.b", "val"))
		// a.b is a string. Trying to treat it as an array should fail.
		err := v.Set("a.b.0", "val2")
		assert.Error(t, err)
	})

	t.Run("NegativeIndexNotMinusOne", func(t *testing.T) {
		v := NewVariables()
		err := v.Set("a.-2", "val")
		assert.Error(t, err)
	})

	t.Run("AppendToNonArray", func(t *testing.T) {
		v := NewVariables()
		assert.NoError(t, v.Set("a", "val"))
		err := v.Set("a.-1", "val2")
		assert.Error(t, err)
	})

	t.Run("MapAsArray", func(t *testing.T) {
		v := NewVariables()
		assert.NoError(t, v.Set("a.b", "val"))
		err := v.Set("a.0", "val2")
		assert.Error(t, err)
	})
}

func TestVariables_BoundaryGet(t *testing.T) {
	v := NewVariables()
	v.Set("a.b.0.c", "val")

	assert.Equal(t, "val", v.Get("a.b.0.c"))
	assert.Nil(t, v.Get("a.b.1.c"))      // Out of bounds
	assert.Nil(t, v.Get("a.x.0.c"))      // Non-existent key
	assert.Nil(t, v.Get("a.b.notint.c")) // Invalid index
	assert.Nil(t, v.Get("a.b.0.c.d"))    // Leaf as map
}

func TestVariables_LargeIndex(t *testing.T) {
	v := NewVariables()
	err := v.Set("a.100", "val")
	assert.NoError(t, err)
	arr := v.Get("a").([]any)
	assert.Equal(t, 101, len(arr))
	assert.Equal(t, "val", arr[100])
	assert.Nil(t, arr[0])
}

func TestVariables_Concurrency(t *testing.T) {
	v := NewVariables()
	const count = 100
	done := make(chan bool)
	for i := 0; i < count; i++ {
		go func(i int) {
			_ = v.Set(fmt.Sprintf("key.%d", i), "val")
			done <- true
		}(i)
	}
	for i := 0; i < count; i++ {
		<-done
	}
	for i := 0; i < count; i++ {
		assert.Equal(t, "val", v.Get(fmt.Sprintf("key.%d", i)))
	}
}
