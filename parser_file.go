package variables

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

func (p *Variables) FromFile(file, namespace string) error {
	return p.FromFileFilter(file, namespace, func(s string) bool {
		return true
	})
}

func (p *Variables) FromFileFilter(file, namespace string, filter func(string) bool) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	switch filepath.Ext(file) {
	case ".yml", ".yaml":
		return p.FromYamlFilter(string(data), namespace, filter)
	case ".prop", ".properties":
		return p.FromPropertiesFilter(string(data), namespace, filter)
	}
	return errors.New("unknown variable type")
}
