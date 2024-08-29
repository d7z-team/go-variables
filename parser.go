package variables

import (
	"github.com/pkg/errors"
	"os"
	"path/filepath"
)

func (p *Variables) FromFile(file, namespace string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	switch filepath.Ext(file) {
	case ".yml", ".yaml":
		return p.FromYaml(string(data), namespace)
	case ".prop", ".properties":
		return p.FromProperties(string(data), namespace)
	}
	return errors.New("unknown variable type")
}
