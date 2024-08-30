package variables

import (
	"fmt"
	"strings"

	"github.com/goccy/go-yaml"
)

func (p *Variables) FromYaml(src, namespace string) error {
	return p.FromYamlFilter(src, namespace, func(key string) bool {
		return true
	})
}

func (p *Variables) FromYamlFilter(src, namespace string, filter func(key string) bool) error {
	namespace = strings.Trim(namespace, ".")
	data := make(yaml.MapSlice, 0)

	err := yaml.UnmarshalWithOptions([]byte(src), &data, yaml.UseOrderedMap())
	if err != nil {
		return err
	}
	output := make([]string, 0)
	err = addAny(&output, namespace, data)
	if err != nil {
		return err
	}
	for _, item := range output {
		key, value, found := strings.Cut(item, "=")
		if !found {
			return fmt.Errorf("variable %s not found", item)
		}
		if filter(key) {
			if err := p.Set(key, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func keyFormat(prefix string, key any) string {
	if prefix == "" {
		return fmt.Sprintf("%v", key)
	} else {
		return fmt.Sprintf("%s.%v", prefix, key)
	}
}

func addAny(dest *[]string, prefix string, node any) error {
	switch data := node.(type) {
	case yaml.MapSlice:
		for _, item := range data {
			if err := addAny(dest, prefix, item); err != nil {
				return err
			}
		}
	case yaml.MapItem:
		if err := addAny(dest, keyFormat(prefix, data.Key), data.Value); err != nil {
			return err
		}
	case []any:
		for i, item := range data {
			if err := addAny(dest, keyFormat(prefix, i), item); err != nil {
				return err
			}
		}
	default:
		*dest = append(*dest, fmt.Sprintf("%s=%s", prefix, data))
	}
	return nil
}
