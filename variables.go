package variables

import (
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
)

type ParseValue func(root *Variables, key string, value string) (any, bool, error)

var (
	parseValues = make([]ParseValue, 0)
	locker      = new(sync.RWMutex)
)

func RegisterParseValue(value ...ParseValue) {
	locker.Lock()
	defer locker.Unlock()
	parseValues = append(parseValues, value...)
}

type (
	Variables map[string]any // 变量
)

func NewVariables() Variables {
	return make(map[string]any)
}

func (p *Variables) Set(key string, value string) error {
	var data any = value
	locker.RLock()
	defer locker.RUnlock()
	for _, f := range parseValues {
		if val, ok := data.(string); ok {
			rel, mOk, err := f(p, key, val)
			if err != nil {
				return err
			}
			if mOk {
				data = rel
			}
		} else {
			break
		}
	}
	return p.SetAny(key, data)
}

func (p *Variables) SetAny(key string, value any) error {
	keys := make([]any, 0, len(*p))
	for _, s := range parseKey(key) {
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
					var next any
					switch keys[1].(type) {
					case int:
						next = make([]any, 0)
					default:
						next = make(map[string]any)
					}
					*child = append(*child, next)
					if asSlice, ok := next.([]any); ok {
						if err := setValue(&asSlice, keys[1:], value); err != nil {
							return err
						}
						(*child)[len(*child)-1] = asSlice
						return nil
					}
					return setValue(next, keys[1:], value)
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
						switch keys[1].(type) {
						case int:
							(*child)[key] = make([]any, 0)
						default:
							(*child)[key] = make(map[string]any)
						}
					}
					next := (*child)[key]
					if asSlice, ok := next.([]any); ok {
						if err := setValue(&asSlice, keys[1:], value); err != nil {
							return err
						}
						(*child)[key] = asSlice
						return nil
					}
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

func (p *Variables) Get(key string) any {
	ok, b := p.GetOK(key)
	if !b {
		return nil
	}
	return ok
}

func (p *Variables) GetOK(key string) (any, bool) {
	return get(p.ToMap(), parseKey(key))
}

func parseKey(key string) []string {
	return strings.Split(key, ".")
}

func get(prefix any, key []string) (any, bool) {
	if len(key) == 0 {
		return prefix, true
	}
	switch child := prefix.(type) {
	case []any:
		index, err := strconv.Atoi(key[0])
		if err != nil {
			return nil, false
		}
		if index < 0 || index >= len(child) {
			return nil, false
		}
		return get(child[index], key[1:])
	case map[string]any:
		next, ok := child[key[0]]
		if !ok {
			return nil, false
		}
		return get(next, key[1:])
	}
	return nil, false
}

func (p *Variables) Clone() (Variables, error) {
	next := NewVariables()
	if err := next.FromStruct(*p, ""); err != nil {
		return nil, err
	}
	return next, nil
}
