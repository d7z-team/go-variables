package variables

import (
	"bytes"
	"strconv"
	"strings"
	"text/template"

	"github.com/pkg/errors"

	"github.com/expr-lang/expr"

	sprig "github.com/go-task/slim-sprig/v3"
)

func tmplParser(p *Variables, _ string, data string) (any, bool, error) {
	parse, err := template.New("tmpl").Funcs(sprig.FuncMap()).Option("missingkey=error").Parse(data)
	if err != nil {
		return "", false, err
	}
	write := bytes.Buffer{}
	defer write.Reset()
	err = parse.Execute(&write, p)
	if err != nil {
		return "", false, err
	}
	return write.String(), true, nil
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
			new(func(...map[string]any) map[string]any),
			new(func(...Variables) map[string]any),
			new(func(...*Variables) map[string]any),
		),
	}
}

func exprParser(p *Variables, _ string, data string) (any, bool, error) {
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

func numberParser(_ *Variables, _ string, src string) (any, bool, error) {
	i, err := strconv.Atoi(src)
	if err == nil {
		return i, true, nil
	}
	float, err := strconv.ParseFloat(src, 64)
	if err == nil {
		return float, true, nil
	}
	b, err := strconv.ParseBool(src)
	if err == nil {
		return b, true, nil
	}
	return nil, false, nil
}

func (p *Variables) Template(data string) (string, error) {
	parser, b, err := tmplParser(p, "", data)
	if err != nil {
		return "", err
	}
	if !b {
		return "", errors.New("template parse error")
	}
	return parser.(string), nil
}

func (p *Variables) Execute(command string) (any, error) {
	compile, err := expr.Compile(command)
	if err != nil {
		return nil, err
	}
	return expr.Run(compile, p)
}

func init() {
	RegisterParseValue(exprParser, tmplParser, numberParser)
}
