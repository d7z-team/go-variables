package variables

import (
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
	if kind >= reflect.Bool && kind <= reflect.Complex128 {
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
	return p.FromStructFilter(s, namespace, filter)
}
