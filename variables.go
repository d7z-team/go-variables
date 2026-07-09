package variables

import (
	"encoding/json"
	"errors"
	"sync"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrTypeConflict      = errors.New("type conflict")
	ErrInvalidPath       = errors.New("invalid path")
	ErrIndexOutOfRange   = errors.New("index out of range")
	ErrUnsupportedFormat = errors.New("unsupported format")
	ErrCycleDetected     = errors.New("cycle detected")
)

type Variables struct {
	mu   sync.RWMutex
	root Value
	opts options
}

func New(opts ...Option) *Variables {
	v := &Variables{
		root: Object(map[string]Value{}),
		opts: defaultOptions(),
	}
	for _, opt := range opts {
		opt(&v.opts)
	}
	return v
}

func (v *Variables) Clone(opts ...Option) *Variables {
	v.mu.RLock()
	defer v.mu.RUnlock()

	cloned := &Variables{
		root: v.root.Clone(),
		opts: v.opts.clone(),
	}
	for _, opt := range opts {
		opt(&cloned.opts)
	}
	return cloned
}

func (v *Variables) Set(path Path, value any) error {
	encoded, err := EncodeValue(value)
	if err != nil {
		return &PathError{Op: "set", Path: path, Err: err}
	}
	return v.SetValue(path, encoded)
}

func (v *Variables) SetValue(path Path, value Value) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if path.IsRoot() {
		v.root = value.Clone()
		return nil
	}
	if err := setAt(&v.root, path, value.Clone()); err != nil {
		return &PathError{Op: "set", Path: path, Err: err}
	}
	return nil
}

func (v *Variables) Append(path Path, value any) error {
	encoded, err := EncodeValue(value)
	if err != nil {
		return &PathError{Op: "append", Path: path, Err: err}
	}
	return v.AppendValue(path, encoded)
}

func (v *Variables) AppendValue(path Path, value Value) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	target, ok := getAt(v.root, path)
	if !ok {
		return &PathError{Op: "append", Path: path, Err: ErrNotFound}
	}
	if target.kind != ArrayValue {
		return &PathError{Op: "append", Path: path, Err: ErrTypeConflict}
	}
	items := append(append([]Value(nil), target.arr...), value.Clone())
	if path.IsRoot() {
		v.root = Value{kind: ArrayValue, arr: items}
		return nil
	}
	if err := setAt(&v.root, path, Value{kind: ArrayValue, arr: items}); err != nil {
		return &PathError{Op: "append", Path: path, Err: err}
	}
	return nil
}

func (v *Variables) Get(path Path) (any, bool) {
	value, ok := v.GetValue(path)
	if !ok {
		return nil, false
	}
	return DecodeValue(value), true
}

func (v *Variables) GetValue(path Path) (Value, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	value, ok := getAt(v.root, path)
	if !ok {
		return Value{}, false
	}
	return value.Clone(), true
}

func (v *Variables) Delete(path Path) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if path.IsRoot() {
		v.root = Object(map[string]Value{})
		return nil
	}
	if err := deleteAt(&v.root, path); err != nil {
		return &PathError{Op: "delete", Path: path, Err: err}
	}
	return nil
}

func (v *Variables) Snapshot() any {
	return DecodeValue(v.SnapshotValue())
}

func (v *Variables) SnapshotValue() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.root.Clone()
}

func (v *Variables) Decode(path Path, dst any) error {
	value, ok := v.GetValue(path)
	if !ok {
		return &PathError{Op: "decode", Path: path, Err: ErrNotFound}
	}
	data, err := json.Marshal(JSONValue(value))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

func setAt(node *Value, path Path, value Value) error {
	seg := path[0]
	last := len(path) == 1
	switch seg.kind {
	case SegmentKey:
		if node.kind != ObjectValue {
			return ErrTypeConflict
		}
		if node.obj == nil {
			node.obj = map[string]Value{}
		}
		if last {
			node.obj[seg.key] = value
			return nil
		}
		child, ok := node.obj[seg.key]
		if !ok || child.kind == NullValue {
			child = newContainer(path[1])
		}
		if err := setAt(&child, path[1:], value); err != nil {
			return err
		}
		node.obj[seg.key] = child
		return nil
	case SegmentIndex:
		if seg.index < 0 {
			return ErrIndexOutOfRange
		}
		if node.kind != ArrayValue {
			return ErrTypeConflict
		}
		for len(node.arr) <= seg.index {
			node.arr = append(node.arr, Null())
		}
		if last {
			node.arr[seg.index] = value
			return nil
		}
		child := node.arr[seg.index]
		if child.kind == NullValue {
			child = newContainer(path[1])
		}
		if err := setAt(&child, path[1:], value); err != nil {
			return err
		}
		node.arr[seg.index] = child
		return nil
	default:
		return ErrInvalidPath
	}
}

func getAt(root Value, path Path) (Value, bool) {
	current := root
	for _, seg := range path {
		switch seg.kind {
		case SegmentKey:
			if current.kind != ObjectValue {
				return Value{}, false
			}
			next, ok := current.obj[seg.key]
			if !ok {
				return Value{}, false
			}
			current = next
		case SegmentIndex:
			if current.kind != ArrayValue || seg.index < 0 || seg.index >= len(current.arr) {
				return Value{}, false
			}
			current = current.arr[seg.index]
		default:
			return Value{}, false
		}
	}
	return current, true
}

func deleteAt(root *Value, path Path) error {
	parentPath := path[:len(path)-1]
	leaf := path[len(path)-1]
	parent, ok := getAt(*root, parentPath)
	if !ok {
		return ErrNotFound
	}
	switch leaf.kind {
	case SegmentKey:
		if parent.kind != ObjectValue {
			return ErrTypeConflict
		}
		if _, ok := parent.obj[leaf.key]; !ok {
			return ErrNotFound
		}
		delete(parent.obj, leaf.key)
	case SegmentIndex:
		if parent.kind != ArrayValue {
			return ErrTypeConflict
		}
		if leaf.index < 0 || leaf.index >= len(parent.arr) {
			return ErrIndexOutOfRange
		}
		parent.arr[leaf.index] = Null()
	default:
		return ErrInvalidPath
	}
	if parentPath.IsRoot() {
		*root = parent
		return nil
	}
	return setAt(root, parentPath, parent)
}

func newContainer(next Segment) Value {
	if next.kind == SegmentIndex {
		return Value{kind: ArrayValue, arr: []Value{}}
	}
	return Object(map[string]Value{})
}
