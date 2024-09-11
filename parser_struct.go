package variables

import "github.com/goccy/go-yaml"

func (p *Variables) FromStruct(src any, namespace string) error {
	marshal, err := yaml.Marshal(src)
	if err != nil {
		return err
	}
	return p.FromYaml(string(marshal), namespace)
}

func (p *Variables) FromStructFilter(src any, namespace string, filter func(key string) bool) error {
	marshal, err := yaml.Marshal(src)
	if err != nil {
		return err
	}
	return p.FromStructFilter(string(marshal), namespace, filter)
}
