package variables

import (
	"fmt"
	"strings"
)

func (v *Variables) LoadArgs(args []string, opts ...LoadOption) error {
	cfg := newLoadOptions(opts)

	root := map[string]any{}
	for _, arg := range args {
		key, raw, ok := strings.Cut(arg, "=")
		if !ok {
			return fmt.Errorf("args: expected key=value, got %q", arg)
		}
		path, err := ParsePath(key)
		if err != nil {
			return err
		}
		value := any(raw)
		if cfg.inferScalars {
			value = inferScalar(raw)
		}
		if err := setAtAny(root, path, value); err != nil {
			return &PathError{Op: "load args", Path: path, Err: err}
		}
	}
	return v.loadValue(root, cfg)
}
