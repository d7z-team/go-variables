package variables

import (
	"strconv"

	"github.com/expr-lang/expr"
)

func covertType(src string) (any, bool) {
	i, err := strconv.Atoi(src)
	if err == nil {
		return i, true
	}
	float, err := strconv.ParseFloat(src, 64)
	if err == nil {
		return float, true
	}
	b, err := strconv.ParseBool(src)
	if err == nil {
		return b, true
	}
	return nil, false
}

func (p *Variables) Execute(command string) (any, error) {
	compile, err := expr.Compile(command)
	if err != nil {
		return nil, err
	}
	return expr.Run(compile, p)
}
