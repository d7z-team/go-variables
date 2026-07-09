package variables

import "sort"

func (v *Variables) Exists(path Path) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	_, ok := getAt(v.root, path)
	return ok
}

func (v *Variables) Len(path Path) (int, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	value, ok := getAt(v.root, path)
	if !ok {
		return 0, false
	}
	switch value.kind {
	case ObjectValue:
		return len(value.obj), true
	case ArrayValue:
		return len(value.arr), true
	default:
		return 0, false
	}
}

func (v *Variables) Keys(path Path) ([]string, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	value, ok := getAt(v.root, path)
	if !ok || value.kind != ObjectValue {
		return nil, false
	}
	keys := make([]string, 0, len(value.obj))
	for key := range value.obj {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys, true
}

func (v *Variables) Children(path Path) (map[string]any, bool) {
	value, ok := v.GetValue(path)
	if !ok || value.kind != ObjectValue {
		return nil, false
	}
	return DecodeValue(value).(map[string]any), true
}

func (v *Variables) Items(path Path) ([]any, bool) {
	value, ok := v.GetValue(path)
	if !ok || value.kind != ArrayValue {
		return nil, false
	}
	return DecodeValue(value).([]any), true
}
