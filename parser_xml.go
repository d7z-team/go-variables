package variables

import (
	"fmt"
	"strings"

	"github.com/clbanning/mxj/v2"
)

func (p *Variables) FromXML(src, namespace string) error {
	return p.FromXMLFilter(src, namespace, func(key string) bool {
		return true
	})
}

func (p *Variables) FromXMLFilter(src, namespace string, filter func(key string) bool) error {
	namespace = strings.Trim(namespace, ".")
	mv, err := mxj.NewMapXml([]byte(src))
	if err != nil {
		return err
	}

	output := make([]string, 0)
	err = addAnyMap(&output, namespace, map[string]any(mv))
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

func addAnyMap(dest *[]string, prefix string, node any) error {
	switch data := node.(type) {
	case map[string]any:
		for k, v := range data {
			if err := addAnyMap(dest, keyFormat(prefix, k), v); err != nil {
				return err
			}
		}
	case []any:
		for i, item := range data {
			if err := addAnyMap(dest, keyFormat(prefix, i), item); err != nil {
				return err
			}
		}
	default:
		*dest = append(*dest, fmt.Sprintf("%s=%v", prefix, data))
	}
	return nil
}
