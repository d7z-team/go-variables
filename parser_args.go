package variables

import (
	"strings"

	"github.com/pkg/errors"
)

func (p *Variables) FromArgs(src []string) error {
	for _, arg := range src {
		key, value, found := strings.Cut(arg, "=")
		if !found {
			return errors.Errorf("格式错误: %s", arg)
		}
		if err := p.Set(key, value); err != nil {
			return errors.Wrapf(err, "插入错误: %v", arg)
		}
	}
	return nil
}
