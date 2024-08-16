package variables

import (
	"bytes"
	"strconv"
	"strings"
	"text/template"

	sprig "github.com/go-task/slim-sprig/v3"
	"github.com/pkg/errors"
)

type Variables map[string]any
type VariablesArray []Variables
type VariablesArrayValue []string

type VariablesTemplate func(string) (string, error)

func NewVariables() Variables {
	return make(map[string]any)
}

func (p *Variables) Template() VariablesTemplate {
	return func(data string) (string, error) {
		parse, err := template.New("tmpl").Funcs(sprig.FuncMap()).Parse(data)
		if err != nil {
			return "", err
		}
		write := bytes.Buffer{}
		defer write.Reset()
		err = parse.Execute(&write, p)
		if err != nil {
			return "", err
		}
		return write.String(), nil
	}
}

func (p *Variables) SetP(key string, value string) {
	err := p.Set(key, value)
	if err != nil {
		panic(err)
	}
}
func (p *Variables) Set(key string, value string) error {
	data, err := p.Template()(value)
	if err != nil {
		return err
	}
	return p.setMap(strings.Split(key, "."), data)
}

func (p *Variables) setEnd(key string, value string) error {
	(*p)[key] = value
	return nil
}
func (p *VariablesArray) setEnd(index int, key []string, value string) error {
	if index == -1 {
		next := NewVariables()
		*p = append(*p, next)
		return next.setMap(key, value)
	}
	if index < 0 {
		return errors.New("未知索引")
	}
	if len(*p) <= index {
		// 其他索引
		next := make(VariablesArray, index+1)
		copy(next, *p)
		next[index] = NewVariables()
		*p = next
		return next[index].setMap(key, value)
	}
	if (*p)[index] == nil {
		(*p)[index] = NewVariables()
	}
	return (*p)[index].setMap(key, value)
}

func (p *VariablesArrayValue) setEnd(index int, value string) error {
	if index == -1 {
		*p = append(*p, value)
		return nil
	}
	if index < 0 {
		return errors.New("未知索引")
	}
	if len(*p) <= index {
		next := make(VariablesArrayValue, index+1)
		copy(next, *p)
		*p = next
	}
	(*p)[index] = value
	return nil
}

func (p *Variables) setMap(key []string, value string) error {
	if len(key) == 1 {
		return p.setEnd(key[0], value)
	}
	index, err := strconv.Atoi(key[1])
	// 填入不存在的内容
	if (*p)[key[0]] == nil {
		if err != nil {
			// map[str][str] 格式
			variables := make(Variables)
			(*p)[key[0]] = &variables
		} else {
			if len(key) == 2 {
				// 尾部 array
				arrayValue := make(VariablesArrayValue, 0)
				(*p)[key[0]] = &arrayValue
			} else {
				// 中部 array
				array := make(VariablesArray, 0)
				(*p)[key[0]] = &array
			}
		}
	}

	if err != nil {
		variables, ok := (*p)[key[0]].(*Variables)
		if !ok {
			return errors.Errorf("节点 %s 无法被分配", key[0])

		}
		return variables.setMap(key[1:], value)
	} else {
		if len(key) == 2 {
			arrayValue, ok := (*p)[key[0]].(*VariablesArrayValue)
			if !ok {
				return errors.Errorf("节点 %s 无法分配为 array value", key[0])
			}
			return arrayValue.setEnd(index, value)
		} else {
			array, ok := (*p)[key[0]].(*VariablesArray)
			if !ok {
				return errors.Errorf("节点 %s 无法分配为 array", key[0])
			}
			return array.setEnd(index, key[2:], value)
		}
	}
}
