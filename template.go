package variables

import (
	"bytes"
	"text/template"

	sprig "github.com/go-task/slim-sprig/v3"
)

type TemplateVariables func(string) (string, error)

func (p *Variables) Template() TemplateVariables {
	return func(data string) (string, error) {
		parse, err := template.New("tmpl").Funcs(sprig.FuncMap()).Option("missingkey=error").Parse(data)
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
