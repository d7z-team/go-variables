package variables

import (
	"encoding/json"
	"reflect"

	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
)

func toYaml(src any) (string, error) {
	if src == nil {
		return "", errors.New("对象为 nil")
	}
	v := reflect.ValueOf(src)
	kind := v.Kind()
	if kind != reflect.Struct &&
		kind != reflect.Map &&
		kind != reflect.Array &&
		kind != reflect.Slice {
		return "", errors.Errorf("对象为基础类型 %T", src)
	}
	marshal, err := yaml.Marshal(src)
	if err != nil {
		return "", nil
	}
	return string(marshal), nil
}

func (p *Variables) FromStruct(src any, namespace string) error {
	s, err := toYaml(src)
	if err != nil {
		return err
	}
	return p.FromYaml(s, namespace)
}

func (p *Variables) FromStructFilter(src any, namespace string, filter func(key string) bool) error {
	s, err := toYaml(src)
	if err != nil {
		return err
	}
	return p.FromYamlFilter(s, namespace, filter)
}

func (p *Variables) Unmarshal(namespace string, any any) error {
	current, exists := p.GetOK(namespace)
	if !exists {
		return errors.Errorf("group %s not found", namespace)
	}
	binary, err := json.Marshal(current)
	if err != nil {
		return err
	}
	return json.Unmarshal(binary, any)
}
