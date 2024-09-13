package variables

import "testing"

func TestExpr(t *testing.T) {
	assertProp(t,
		`{"foo":{"foo1":"bar1","foo2":"bar2"},"foo1":{"foo1":"bar1"},"foo2":{"foo2":"bar2"}}`, `
foo1.foo1=bar1
foo2.foo2=bar2
foo=${{concat(foo1 ,foo2)}}
`)
}
