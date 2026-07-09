package variables

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

type Format string

const (
	FormatJSON       Format = "json"
	FormatYAML       Format = "yaml"
	FormatXML        Format = "xml"
	FormatProperties Format = "properties"
)

type MergeMode int

const (
	Replace MergeMode = iota
	DeepMerge
	ErrorOnConflict
)

type LoadOption func(*loadOptions)

type loadOptions struct {
	prefix       Path
	mergeMode    MergeMode
	filter       func(Path, any) bool
	inferScalars bool
}

func WithPrefix(path Path) LoadOption {
	return func(opts *loadOptions) {
		opts.prefix = path
	}
}

func WithMergeMode(mode MergeMode) LoadOption {
	return func(opts *loadOptions) {
		opts.mergeMode = mode
	}
}

func WithFilter(filter func(Path, any) bool) LoadOption {
	return func(opts *loadOptions) {
		opts.filter = filter
	}
}

func WithScalarInference() LoadOption {
	return func(opts *loadOptions) {
		opts.inferScalars = true
	}
}

func (v *Variables) Load(r io.Reader, format Format, opts ...LoadOption) error {
	cfg := newLoadOptions(opts)

	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	var value any
	switch format {
	case FormatJSON:
		value, err = decodeJSON(data)
	case FormatYAML:
		value, err = decodeYAML(data)
	case FormatXML:
		value, err = decodeXML(data)
	case FormatProperties:
		value, err = decodeProperties(string(data), cfg.inferScalars)
	default:
		return fmt.Errorf("%w: %s", ErrUnsupportedFormat, format)
	}
	if err != nil {
		return err
	}
	return v.loadValue(value, cfg)
}

func (v *Variables) LoadString(src string, format Format, opts ...LoadOption) error {
	return v.Load(strings.NewReader(src), format, opts...)
}

func (v *Variables) LoadFile(file string, opts ...LoadOption) error {
	format, err := FormatFromFile(file)
	if err != nil {
		return err
	}
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	return v.Load(f, format, opts...)
}

func FormatFromFile(file string) (Format, error) {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".json":
		return FormatJSON, nil
	case ".yaml", ".yml":
		return FormatYAML, nil
	case ".xml":
		return FormatXML, nil
	case ".properties", ".prop":
		return FormatProperties, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedFormat, file)
	}
}

func (v *Variables) loadValue(value any, cfg loadOptions) error {
	normalized, err := EncodeValue(value)
	if err != nil {
		return &PathError{Op: "load", Path: cfg.prefix, Err: err}
	}
	if cfg.filter != nil {
		filtered, ok := filterValue(cfg.prefix, normalized, cfg.filter)
		if !ok {
			return nil
		}
		normalized = filtered
	}

	switch cfg.mergeMode {
	case Replace:
		return v.SetValue(cfg.prefix, normalized)
	case ErrorOnConflict:
		if _, ok := v.Get(cfg.prefix); ok && !cfg.prefix.IsRoot() {
			return &PathError{Op: "load", Path: cfg.prefix, Err: ErrTypeConflict}
		}
		if cfg.prefix.IsRoot() {
			if rootHasValue(v.SnapshotValue()) {
				return &PathError{Op: "load", Path: cfg.prefix, Err: ErrTypeConflict}
			}
		}
		return v.SetValue(cfg.prefix, normalized)
	case DeepMerge:
		return v.mergeValue(cfg.prefix, normalized)
	default:
		return fmt.Errorf("unknown merge mode: %d", cfg.mergeMode)
	}
}

func decodeJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("json: multiple top-level values")
		}
		return nil, err
	}
	return EncodeValue(value)
}

func newLoadOptions(opts []LoadOption) loadOptions {
	cfg := loadOptions{}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func rootHasValue(value Value) bool {
	switch value.kind {
	case ObjectValue:
		return len(value.obj) > 0
	case ArrayValue:
		return len(value.arr) > 0
	case NullValue:
		return false
	default:
		return true
	}
}

func decodeYAML(data []byte) (any, error) {
	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func (v *Variables) mergeValue(path Path, value Value) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if path.IsRoot() {
		v.root = mergeNodes(v.root, value)
		return nil
	}
	current, ok := getAt(v.root, path)
	if !ok {
		return setAt(&v.root, path, value.Clone())
	}
	merged := mergeNodes(current, value)
	return setAt(&v.root, path, merged)
}

func mergeNodes(dst Value, src Value) Value {
	if dst.kind == ObjectValue && src.kind == ObjectValue {
		out := dst.Clone()
		for key, srcValue := range src.obj {
			if dstValue, ok := out.obj[key]; ok {
				out.obj[key] = mergeNodes(dstValue, srcValue)
			} else {
				out.obj[key] = srcValue.Clone()
			}
		}
		return out
	}
	return src.Clone()
}

func filterValue(path Path, value Value, filter func(Path, any) bool) (Value, bool) {
	if !filter(path, DecodeValue(value)) {
		return Value{}, false
	}
	switch value.kind {
	case ObjectValue:
		out := make(map[string]Value, len(value.obj))
		for key, child := range value.obj {
			filtered, ok := filterValue(appendPath(path, Key(key)), child, filter)
			if ok {
				out[key] = filtered
			}
		}
		return Value{kind: ObjectValue, obj: out}, true
	case ArrayValue:
		out := make([]Value, 0, len(value.arr))
		for i, child := range value.arr {
			filtered, ok := filterValue(appendPath(path, Index(i)), child, filter)
			if ok {
				out = append(out, filtered)
			}
		}
		return Value{kind: ArrayValue, arr: out}, true
	default:
		return value, true
	}
}

func appendPath(path Path, segment Segment) Path {
	next := make(Path, len(path), len(path)+1)
	copy(next, path)
	return append(next, segment)
}
