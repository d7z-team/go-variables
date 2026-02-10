package variables

import (
	"encoding/json"
	"fmt"
	"strings"
)

func (p *Variables) FromJson(src, namespace string) error {
	return p.FromJsonFilter(src, namespace, func(key string) bool {
		return true
	})
}

func (p *Variables) FromJsonFilter(src, namespace string, filter func(key string) bool) error {
	namespace = strings.Trim(namespace, ".")
	var data any
	err := json.Unmarshal([]byte(src), &data)
	if err != nil {
		return err
	}

	output := make([]string, 0)
	err = addAnyMap(&output, namespace, data)
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
