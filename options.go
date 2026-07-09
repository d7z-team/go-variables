package variables

type Option func(*options)

type options struct {
	funcs         map[string]Function
	functionSpecs map[string]FunctionSpec
	templateFuncs map[string]any
}

type Function func(FunctionContext, []Value) (Value, error)

type FunctionContext struct {
	Root    Value
	Current Value
	Path    Path
}

func defaultOptions() options {
	return options{
		funcs:         map[string]Function{},
		functionSpecs: map[string]FunctionSpec{},
		templateFuncs: map[string]any{},
	}
}

func (opts options) clone() options {
	cloned := options{
		funcs:         make(map[string]Function, len(opts.funcs)),
		functionSpecs: make(map[string]FunctionSpec, len(opts.functionSpecs)),
		templateFuncs: make(map[string]any, len(opts.templateFuncs)),
	}
	for name, fn := range opts.funcs {
		cloned.funcs[name] = fn
	}
	for name, spec := range opts.functionSpecs {
		cloned.functionSpecs[name] = spec
	}
	for name, fn := range opts.templateFuncs {
		cloned.templateFuncs[name] = fn
	}
	return cloned
}

func WithFunction(name string, fn Function) Option {
	return func(opts *options) {
		mustAllowCustomFunction(name)
		opts.funcs[name] = fn
		opts.functionSpecs[name] = unknownFunctionSpec(name, fn)
	}
}

func WithGoFunction(name string, fn any) Option {
	return func(opts *options) {
		mustAllowCustomFunction(name)
		opts.funcs[name] = AdaptFunction(fn)
		opts.functionSpecs[name] = goFunctionSpec(name, fn)
		opts.templateFuncs[name] = fn
	}
}

func WithTypedFunction(name string, spec FunctionSpec) Option {
	return func(opts *options) {
		mustAllowCustomFunction(name)
		if spec.Runtime == nil {
			panic("typed function runtime is required for " + name)
		}
		spec.Name = name
		opts.funcs[name] = spec.Runtime
		opts.functionSpecs[name] = spec
	}
}

func WithFunctions(funcs map[string]Function) Option {
	return func(opts *options) {
		for name, fn := range funcs {
			mustAllowCustomFunction(name)
			opts.funcs[name] = fn
			opts.functionSpecs[name] = unknownFunctionSpec(name, fn)
		}
	}
}

func WithGoFunctions(funcs map[string]any) Option {
	return func(opts *options) {
		for name, fn := range funcs {
			mustAllowCustomFunction(name)
			opts.funcs[name] = AdaptFunction(fn)
			opts.functionSpecs[name] = goFunctionSpec(name, fn)
			opts.templateFuncs[name] = fn
		}
	}
}

func mustAllowCustomFunction(name string) {
	if _, ok := builtinFunctionSpecs[name]; ok {
		panic("custom function cannot override built-in function " + name)
	}
}
