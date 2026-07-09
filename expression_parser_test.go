package variables

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseExpressionAcceptsNewSyntax(t *testing.T) {
	tests := []string{
		"app.name",
		"app?.name",
		"servers[0].host",
		"servers[-1].host",
		"servers.host",
		"servers?[enabled].host",
		`servers?[enabled && cpu >= 2].host`,
		`servers?[region in ["us", "eu"]].host`,
		`servers?[region in ['us', 'eu']].host`,
		`servers?[region == $.app.defaultRegion].host`,
		`sum(servers?[enabled].cpu) / count(servers?[enabled])`,
		`{"name": app.name, "hosts": servers.host}`,
	}

	for _, src := range tests {
		t.Run(src, func(t *testing.T) {
			expr, err := ParseExpression(src)
			require.NoError(t, err)
			require.Equal(t, src, expr.String())
		})
	}
}

func TestParseExpressionRejectsUnsupportedAndInvalidSyntax(t *testing.T) {
	tests := []string{
		"servers[*].host",
		"servers[?(enabled)].host",
		"**.host",
		"query(\"servers.host\")",
		"servers | count",
		"app..name",
		"sum(servers.cpu,)",
		`["a",]`,
		`{"a": 1,}`,
		`{"a": 1, "a": 2}`,
	}

	for _, src := range tests {
		t.Run(src, func(t *testing.T) {
			_, err := ParseExpression(src)
			require.Error(t, err)
		})
	}
}

func TestMustExpressionPanicsOnInvalidExpression(t *testing.T) {
	require.Panics(t, func() {
		_ = MustExpression("app..name")
	})
}

func TestParseExpressionValidatesFunctionSetWhenProvided(t *testing.T) {
	_, err := ParseExpression(`custom(app.name)`, WithExpressionFunctions(map[string]any{
		"custom": func(any) any { return nil },
	}))
	require.NoError(t, err)

	_, err = ParseExpression(`missing(app.name)`, WithExpressionFunctions(map[string]any{}))
	require.Error(t, err)
}
