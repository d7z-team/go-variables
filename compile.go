package variables

import (
	"reflect"
	"strconv"

	"github.com/expr-lang/expr"
)

// Compile 对象类型转换
func (p *Variables) Compile() error {
	var err error
	for key, value := range *p {
		switch value.(type) {
		case *Variables:
			if err = value.(*Variables).Compile(); err != nil {
				return err
			}
		case *VariablesArray:
			for _, variables := range *value.(*VariablesArray) {
				if err = variables.Compile(); err != nil {
					return err
				}
			}
		case *VariablesArrayValue:
			var target string
			dst := make([]any, 0)
			for _, variables := range *value.(*VariablesArrayValue) {
				a, b := covertType(variables)
				if !b {
					break
				}
				typeOf := reflect.TypeOf(a)
				if target == "" {
					target = typeOf.String()
				}
				if typeOf.String() != target {
					break
				}
				dst = append(dst, a)
			}
			if len(dst) == len(*value.(*VariablesArrayValue)) {
				(*p)[key] = &dst
			}
		default:
			if value, ok := value.(string); ok {
				a, b := covertType(value)
				if b {
					(*p)[key] = a
				}
			}
		}
	}
	return nil
}

func covertType(src string) (any, bool) {
	i, err := strconv.Atoi(src)
	if err == nil {
		return i, true
	}
	float, err := strconv.ParseFloat(src, 64)
	if err == nil {
		return float, true
	}
	b, err := strconv.ParseBool(src)
	if err == nil {
		return b, true
	}
	return nil, false
}

func (p *Variables) Execute(command string) (any, error) {
	compile, err := expr.Compile(command)
	if err != nil {
		return nil, err
	}
	return expr.Run(compile, p)
}
