# go variables

Typed configuration variables for Go.

## Features

- Load JSON, YAML, XML, properties, args, files, and structs into a typed data tree.
- Store data internally as a strict `Value` tree.
- Preserve numbers with `math/big.Int` and `math/big.Float`.
- Access data with explicit structured paths such as `app.servers[0].host`.
- Use quoted path keys for special names: `app["key.with.dot"]`.
- Render templates and evaluate precompiled expressions only when explicitly requested.
- Return snapshots and values as deep copies to avoid accidental mutation.

## Example

```go
package main

import (
	"fmt"
	"strings"

	"gopkg.d7z.net/go-variables"
)

func main() {
	v := variables.New()

	_ = v.Load(strings.NewReader(`
app:
  name: demo
  greeting: "hello {{.app.name}}"
servers:
  - host: localhost
`), variables.FormatYAML)

	_ = v.Interpolate()

	name, _ := v.Get(variables.MustPath("app.name"))
	host, _ := v.Get(variables.MustPath("servers[0].host"))
	greeting, _ := v.Get(variables.MustPath("app.greeting"))

	fmt.Println(name)
	fmt.Println(host)
	fmt.Println(greeting)
}
```

## Loading

```go
_ = v.Load(reader, variables.FormatJSON)
_ = v.LoadString(`{"app":{"name":"demo"}}`, variables.FormatJSON)
_ = v.LoadFile("config.yaml")
_ = v.LoadArgs([]string{"app.name=demo"})
_ = v.LoadStruct(config)
```

Use `WithScalarInference()` only when string inputs such as properties or args should infer bool, number, or null values.

Direct `Set` and `Append` inputs must encode into the tree model: null, bool, string, number, object, or array. Structs and ordinary pointers are rejected; use `LoadStruct` for explicit struct conversion.

## Paths

- Empty path means root: `variables.Root()`.
- Object keys use dot syntax: `app.name`.
- Array indexes use brackets: `servers[0]`.
- Keys containing dots or special characters use quoted brackets: `app["key.with.dot"]`.
- Paths can be built without parsing strings: `variables.Root().Child(variables.Key("servers"), variables.Index(0))`.

## Expressions

```go
exists := v.Exists(variables.MustPath("app.name"))
count, ok := v.Len(variables.MustPath("servers"))
keys, ok := v.Keys(variables.MustPath("app"))
children, ok := v.Children(variables.MustPath("app"))
items, ok := v.Items(variables.MustPath("servers"))
```

Use expressions for multi-node selection and computation. Paths are variables, arrays project fields automatically, filters use `?[...]`, and optional access uses `?.`.

```go
hosts, err := v.CompileExpression("servers.host")
enabled, err := v.CompileExpression("servers?[enabled && cpu >= 2].host")
totalCPU, err := v.CompileExpression("sum(servers.cpu)")

matches, err := v.Select(hosts)
values, err := v.SelectValues(enabled)
first, ok, err := v.First(enabled)
total, err := v.Eval(totalCPU)
```

Expressions support `+ - * / %`, comparisons, `&& || !`, `in`, arrays, objects, `$` root access, and pure functions such as `count`, `sum`, `avg`, `contains`, `sort`, and `sortBy`. Compile expressions once with `CompileExpression` or `(*Variables).CompileExpression` and reuse them across evaluations.

Compilation performs static checks when types are clear from literals, function signatures, or the current variable tree. For example, invalid arithmetic, bad function arity, missing known fields, and invalid sort or aggregate inputs fail before evaluation. Unknown dynamic values are still checked at runtime.

Integers evaluate as `*big.Int`; decimal values evaluate as `*big.Float`.

Function calls can also use method-call syntax. `a.x(b)` is equivalent to `x(a, b)`, so chained expressions stay compact:

```go
fastest, err := v.CompileExpression(`servers?[enabled].sortByDesc("cpu").host.first()`)
total, err := v.CompileExpression(`servers.cpu.sum()`)
hasHost, err := v.CompileExpression(`servers.host.contains("localhost")`)
```

External functions can be registered as `Function` values or ordinary Go functions. Go function signatures are used by expression compilation; use `WithTypedFunction` when a low-level `Function` also needs explicit static checks.

```go
v := variables.New(
	variables.WithGoFunction("prefix", func(s string, p string) string {
		return p + s
	}),
)
value, err := v.EvalString(`app.name.prefix("service-")`)
```

## Copying

```go
copy := v.Clone()
snapshot := v.Snapshot()
typed := v.SnapshotValue()
```

`Clone` returns an independent variables container. `Snapshot` returns decoded Go values, while `SnapshotValue` returns a detached `Value` tree.

## License

MIT
