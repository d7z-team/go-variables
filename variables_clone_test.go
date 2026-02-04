package variables

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVariables_Clone(t *testing.T) {
	v := NewVariables()
	v.SetAny("a", map[string]any{"b": "c"})
	v.SetAny("d", []any{1, 2, 3})

	v2, err := v.Clone()
	assert.NoError(t, err)

	// 修改克隆后的对象
	v2.Set("a.b", "modified")
	v2.Set("d.0", "99")

	// 原对象不应改变
	assert.Equal(t, "c", v.Get("a.b"))
	assert.Equal(t, 1, v.Get("d.0"))

	// 克隆对象已改变
	assert.Equal(t, "modified", v2.Get("a.b"))
	assert.Equal(t, 99, v2.Get("d.0"))
}
