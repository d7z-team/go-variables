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
	data := make(yaml.MapSlice, 0)

	err := yaml.UnmarshalWithOptions([]byte(src), &data, yaml.UseOrderedMap())
	if err != nil {
		return err
	}
	output := make([]string, 0)
	for _, datum := range data {
		err = addItem(&output, namespace, datum)
		if err != nil {
			return err
		}
	}
	for _, item := range output {
		before, after, found := strings.Cut(item, "=")
		if !found {
			return fmt.Errorf("variable %s not found", item)
		}
		if filter(before) {
			if err := p.Set(before, after); err != nil {
				return err
			}
		}
	}
	return nil
}

func addItem(dest *[]string, prefix string, node yaml.MapItem) error {
	key := strings.TrimPrefix(fmt.Sprintf("%s.%s", prefix, node.Key), ".")
	switch node.Value.(type) {
	case yaml.MapItem:
		return addItem(dest, key, node.Value.(yaml.MapItem))
	case yaml.MapSlice:
		for _, datum := range node.Value.(yaml.MapSlice) {
			err := addItem(dest, key, datum)
			if err != nil {
				return err
			}
		}
	case []any:
		for i, data := range node.Value.([]any) {
			*dest = append(*dest, fmt.Sprintf("%s.%d=%s", key, i, data))
		}
	default:
		*dest = append(*dest, fmt.Sprintf("%s=%s", key, node.Value))
	}
	return nil
}
