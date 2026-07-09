package variables

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"text/template"

	sprig "github.com/go-task/slim-sprig/v3"
)

type EvalOption func(*evalOptions)

type evalOptions struct {
	recursive bool
	maxDepth  int
}

func WithRecursiveInterpolation() EvalOption {
	return func(opts *evalOptions) {
		opts.recursive = true
	}
}

func WithMaxDepth(depth int) EvalOption {
	return func(opts *evalOptions) {
		opts.maxDepth = depth
	}
}

func (v *Variables) Render(tmpl string) (string, error) {
	return renderTemplate(tmpl, v.Snapshot(), v.opts.templateFuncs)
}

func (v *Variables) Interpolate(opts ...EvalOption) error {
	cfg := evalOptions{maxDepth: 32}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.maxDepth <= 0 {
		cfg.maxDepth = 32
	}

	current := v.SnapshotValue()
	for depth := 0; ; depth++ {
		next, err := evaluateNode(current, current, v.opts)
		if err != nil {
			return err
		}
		if !cfg.recursive || !containsEvaluation(next) {
			return v.SetValue(Root(), next)
		}
		if reflect.DeepEqual(current, next) {
			return ErrCycleDetected
		}
		if depth+1 >= cfg.maxDepth {
			return ErrCycleDetected
		}
		current = next
	}
}

func evaluateNode(value Value, env Value, opts options) (Value, error) {
	switch value.kind {
	case ObjectValue:
		out := make(map[string]Value, len(value.obj))
		for key, child := range value.obj {
			evaluated, err := evaluateNode(child, env, opts)
			if err != nil {
				return Value{}, err
			}
			out[key] = evaluated
		}
		return Value{kind: ObjectValue, obj: out}, nil
	case ArrayValue:
		out := make([]Value, len(value.arr))
		for i, child := range value.arr {
			evaluated, err := evaluateNode(child, env, opts)
			if err != nil {
				return Value{}, err
			}
			out[i] = evaluated
		}
		return Value{kind: ArrayValue, arr: out}, nil
	case StringValue:
		return evaluateString(value.s, env, opts)
	default:
		return value.Clone(), nil
	}
}

func renderTemplate(tmpl string, env any, funcs map[string]any) (string, error) {
	funcMap := sprig.FuncMap()
	for name, fn := range funcs {
		funcMap[name] = fn
	}
	parsed, err := template.New("variables").Funcs(funcMap).Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := parsed.Execute(&out, env); err != nil {
		return "", err
	}
	return out.String(), nil
}

func evaluateString(src string, env Value, opts options) (Value, error) {
	if expression, ok := wholeExpressionPlaceholder(src); ok {
		return evalTemplateExpression(expression, env, opts)
	}
	if strings.Contains(src, "${{") {
		rendered, err := renderExpressionPlaceholders(src, env, opts)
		if err != nil {
			return Value{}, err
		}
		src = rendered
	}
	if strings.Contains(src, "{{") {
		rendered, err := renderTemplate(src, DecodeValue(env), opts.templateFuncs)
		if err != nil {
			return Value{}, err
		}
		return String(rendered), nil
	}
	return String(src), nil
}

func wholeExpressionPlaceholder(src string) (string, bool) {
	trimmed := strings.TrimSpace(src)
	if !strings.HasPrefix(trimmed, "${{") {
		return "", false
	}
	end, ok := findExpressionPlaceholderEnd(trimmed, 3)
	if !ok || strings.TrimSpace(trimmed[end+2:]) != "" {
		return "", false
	}
	body := strings.TrimSpace(trimmed[3:end])
	return body, body != ""
}

func renderExpressionPlaceholders(src string, env Value, opts options) (string, error) {
	var out strings.Builder
	for {
		start := strings.Index(src, "${{")
		if start < 0 {
			out.WriteString(src)
			return out.String(), nil
		}
		out.WriteString(src[:start])
		end, ok := findExpressionPlaceholderEnd(src, start+3)
		if !ok {
			return "", fmt.Errorf("unclosed expression placeholder")
		}
		expression := strings.TrimSpace(src[start+3 : end])
		if expression == "" {
			return "", fmt.Errorf("empty expression placeholder")
		}
		value, err := evalTemplateExpression(expression, env, opts)
		if err != nil {
			return "", err
		}
		out.WriteString(FormatValue(value))
		src = src[end+2:]
	}
}

func findExpressionPlaceholderEnd(src string, start int) (int, bool) {
	escaped := false
	var quote byte
	for i := start; i < len(src)-1; i++ {
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			switch src[i] {
			case '\\':
				escaped = true
			case quote:
				quote = 0
			}
			continue
		}
		if src[i] == '"' || src[i] == '\'' {
			quote = src[i]
			continue
		}
		if src[i] == '}' && src[i+1] == '}' {
			return i, true
		}
	}
	return 0, false
}

func evalTemplateExpression(src string, env Value, opts options) (Value, error) {
	expr, err := CompileExpression(src, WithRootValue(env), WithFunctionSpecs(opts.functionSpecs))
	if err != nil {
		return Value{}, err
	}
	ctx := evalContext{root: env, current: env, path: Root(), opts: opts}
	result, err := expr.root.eval(&ctx)
	if err != nil {
		return Value{}, err
	}
	return result.output().Clone(), nil
}

func containsEvaluation(value Value) bool {
	switch value.kind {
	case ObjectValue:
		for _, child := range value.obj {
			if containsEvaluation(child) {
				return true
			}
		}
	case ArrayValue:
		for _, child := range value.arr {
			if containsEvaluation(child) {
				return true
			}
		}
	case StringValue:
		trimmed := strings.TrimSpace(value.s)
		return strings.Contains(value.s, "{{") ||
			(strings.HasPrefix(trimmed, "${{") && strings.HasSuffix(trimmed, "}}"))
	}
	return false
}
