package variables

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type (
	Variables map[string]any // 变量
)

func NewVariables() Variables {
	return make(map[string]any)
}

func (p *Variables) Set(key string, value string) error {
	data, err := p.Template()(value)
	if err != nil {
		return err
	}

	target, ok := covertType(data)
	if !ok {
		target = data
	}
	return p.SetAny(key, target)
}

func (p *Variables) SetAny(key string, value any) error {
	keys := make([]any, 0, len(*p))
	for _, s := range strings.Split(key, ".") {
		index, err := strconv.Atoi(s)
		if err != nil {
			keys = append(keys, s)
		} else {
			keys = append(keys, index)
		}
	}
	var tmp map[string]any
	tmp = *p
	return setValue(
		tmp,
		keys,
		value,
	)
}

func setValue(prefix any, keys []any, value any) error {
	isLast := len(keys) == 1
	switch key := keys[0].(type) {
	case string:
		if child, ok := prefix.(map[string]any); ok {
			if isLast {
				// 结束
				child[key] = value
			} else {
				// 委托下一级
				switch keys[1].(type) {
				case int:
					current := make([]any, 0)
					if child[key] != nil {
						current, ok = child[key].([]any)
						if !ok {
							return errors.Errorf("invalid type %T, expected []any", child[key])
						}
					}
					if err := setValue(&current, keys[1:], value); err != nil {
						return err
					} else {
						child[key] = current
						return nil
					}
				case string:
					current := make(map[string]any)
					if child[key] != nil {
						current, ok = child[key].(map[string]any)
						if !ok {
							return errors.Errorf("invalid type %T, expected map[string]any", child[key])
						}
					}
					child[key] = current
					return setValue(current, keys[1:], value)
				default:
					return errors.Errorf("未知的 key 类型 %T", keys[1])
				}
			}
		} else {
			return errors.Errorf("当前无法容纳 map, key: %t", prefix)
		}
	case int:
		if child, ok := prefix.(*[]any); ok {
			if key == -1 {
				// 追加模式
				if isLast {
					*child = append(*child, value)
				} else {
					empty := make(map[string]any)
					*child = append(*child, empty)
					return setValue(empty, keys[1:], value)
				}
			} else if key >= 0 {
				if len(*child) <= key {
					// 处理长度不够的问题
					nextVar := make([]any, key+1)
					copy(nextVar, *child)
					*child = nextVar
				}
				if isLast {
					(*child)[key] = value
				} else {
					if (*child)[key] == nil {
						empty := make(map[string]any)
						(*child)[key] = empty
					}
					next := (*child)[key]
					defer func() {
						(*child)[key] = next
					}()
					return setValue(next, keys[1:], value)
				}
			} else {
				return errors.New("错误的 index")
			}
		} else {
			return errors.New("当然无法容纳 array")
		}
	default:
		return errors.Errorf("未知类型 key: %T", prefix)
	}
	return nil
}

func (p *Variables) ToMap() map[string]any {
	return *p
}
