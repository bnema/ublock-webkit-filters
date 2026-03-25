package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFromOptionAsDomainAlias(t *testing.T) {
	p := New()
	filters, err := p.Parse(strings.NewReader(
		`$image,3p,from=pussyspace.com|pussyspace.net`,
	))
	assert.NoError(t, err)
	assert.Len(t, filters, 1)
	assert.Equal(t, []string{"pussyspace.com", "pussyspace.net"}, filters[0].Options.Domains)
}

func TestParseFromOptionWithExcludes(t *testing.T) {
	p := New()
	filters, err := p.Parse(strings.NewReader(
		`*$script,3p,from=scnlog.me|~example.com`,
	))
	assert.NoError(t, err)
	assert.Len(t, filters, 1)
	assert.Equal(t, []string{"scnlog.me"}, filters[0].Options.Domains)
	assert.Equal(t, []string{"example.com"}, filters[0].Options.ExcludeDomains)
}
