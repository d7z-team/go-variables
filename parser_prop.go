package variables

import (
	"strings"

	"github.com/pkg/errors"
)

func (p *Variables) FromProperties(src, namespace string) error {
	if namespace != "" {
		namespace = strings.Trim(namespace, ".") + "."
	}
	lines := strings.Split(src, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" || strings.TrimSpace(line)[0] == '#' {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			return errors.Errorf("properties 格式错误，位于 %d 行", i)
		}
		for strings.HasSuffix(value, "\\") && i+1 < len(lines) {
			i = i + 1
			value = value[:len(value)-1] + lines[i]
		}
		value = strings.ReplaceAll(value, "\\n", "\n")
		if err := p.Set(namespace+key, value); err != nil {
			return errors.Wrapf(err, "properties 格式错误，位于 %d 行", i)
		}
	}
	return nil
}
