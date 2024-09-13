package variables

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/expr-lang/expr"

	sprig "github.com/go-task/slim-sprig/v3"
)

func (p *Variables) Template(data string) (string, error) {
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

func exprOptions() []expr.Option {
	return []expr.Option{
		expr.Function(
			"concat",
			func(params ...any) (any, error) {
				result := NewVariables()
				for _, param := range params {
					err := result.FromStruct(param, "")
					if err != nil {
						return nil, err
					}
				}
				return result.ToMap(), nil
			},
			new(func(map[string]any, map[string]any) map[string]any),
		),
	}
}

func (p *Variables) Expr(data string) (any, bool, error) {
	trimData := strings.TrimSpace(data)
	if strings.HasPrefix(trimData, "${{") && strings.HasSuffix(trimData, "}}") {
		compile, err := expr.Compile(strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimData, "${{"), "}}")), exprOptions()...)
		if err != nil {
			return nil, true, err
		}
		result, err := expr.Run(compile, p)
		if err != nil {
			return nil, true, err
		}
		return result, true, nil
	} else {
		return nil, false, nil
	}
}
