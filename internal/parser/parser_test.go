package parser

import (
	"strings"
	"testing"

	"github.com/bnema/ublock-webkit-filters/internal/models"
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

func TestDenyallowIsSkipped(t *testing.T) {
	p := New()
	filters, err := p.Parse(strings.NewReader(
		`*$script,3p,denyallow=cdn77.org|google.com|gstatic.com,domain=pingit.com`,
	))
	assert.NoError(t, err)
	// Filter should be skipped (unsupported option)
	assert.Empty(t, filters)
	assert.Equal(t, 1, p.Stats().Unsupported)
}

func TestDenyallowWithFromIsSkipped(t *testing.T) {
	p := New()
	filters, err := p.Parse(strings.NewReader(
		`$image,3p,denyallow=fpbns.net|globalcdn.co,from=pussyspace.com`,
	))
	assert.NoError(t, err)
	assert.Empty(t, filters)
	assert.Equal(t, 1, p.Stats().Unsupported)
}

func TestRegexDomainIsSkipped(t *testing.T) {
	p := New()
	filters, err := p.Parse(strings.NewReader(
		`||example.com^$script,from=/img[a-z]{3,5}\.buzz/`,
	))
	assert.NoError(t, err)
	assert.Empty(t, filters, "filter with regex domain value must be skipped")
	assert.Equal(t, 1, p.Stats().Unsupported)
}

func TestIPAddressOptionIsSkipped(t *testing.T) {
	p := New()
	filters, err := p.Parse(strings.NewReader(
		`*$doc,ipaddress=178.16.53.131`,
	))
	assert.NoError(t, err)
	assert.Empty(t, filters, "ipaddress= filters must be skipped")
	assert.Equal(t, 1, p.Stats().Unsupported)
}

func TestStrict3POptionIsSkipped(t *testing.T) {
	p := New()
	filters, err := p.Parse(strings.NewReader(
		`*$strict3p,ipaddress=0.0.0.0,domain=~0.0.0.0|~127.0.0.1|~[::1]|~[::]|~local|~localhost`,
	))
	assert.NoError(t, err)
	assert.Empty(t, filters, "strict3p filters must be skipped")
	assert.Equal(t, 1, p.Stats().Unsupported)
}

func TestNegatedResourceTypeIsSkipped(t *testing.T) {
	p := New()
	filters, err := p.Parse(strings.NewReader(
		`.ar/ads/$~xmlhttprequest`,
	))
	assert.NoError(t, err)
	assert.Empty(t, filters, "negated resource types must be skipped")
	assert.Equal(t, 1, p.Stats().Unsupported)
}

func TestAllOptionIsAcceptedAsNoOp(t *testing.T) {
	p := New()
	filters, err := p.Parse(strings.NewReader(
		`||example.com^$all`,
	))
	assert.NoError(t, err)
	assert.Len(t, filters, 1)
	assert.Empty(t, filters[0].Options.ResourceTypes)
}

func TestPingOptionIsMappedPrecisely(t *testing.T) {
	p := New()
	filters, err := p.Parse(strings.NewReader(
		`$ping,third-party`,
	))
	assert.NoError(t, err)
	assert.Len(t, filters, 1)
	assert.Equal(t, []string{models.ResourcePing}, filters[0].Options.ResourceTypes)
}

func TestBeaconOptionIsSkipped(t *testing.T) {
	p := New()
	filters, err := p.Parse(strings.NewReader(
		`$beacon,third-party`,
	))
	assert.NoError(t, err)
	assert.Empty(t, filters, "beacon filters must be skipped because WebKit has no precise beacon type")
	assert.Equal(t, 1, p.Stats().Unsupported)
}

func TestObjectOptionIsSkipped(t *testing.T) {
	p := New()
	filters, err := p.Parse(strings.NewReader(
		`||example.com^$object`,
	))
	assert.NoError(t, err)
	assert.Empty(t, filters, "object filters must be skipped because WebKit has no precise object type")
	assert.Equal(t, 1, p.Stats().Unsupported)
}

func TestLatestWebKitResourceTypesAreUsed(t *testing.T) {
	p := New()
	filters, err := p.Parse(strings.NewReader(
		`||example.com^$doc
||example.com^$subdocument
||example.com^$xhr
||example.com^$websocket
||example.com^$other`,
	))
	assert.NoError(t, err)
	assert.Len(t, filters, 5)
	assert.Equal(t, []string{models.ResourceTopDocument}, filters[0].Options.ResourceTypes)
	assert.Equal(t, []string{models.ResourceChildDocument}, filters[1].Options.ResourceTypes)
	assert.Equal(t, []string{models.ResourceFetch}, filters[2].Options.ResourceTypes)
	assert.Equal(t, []string{models.ResourceWebSocket}, filters[3].Options.ResourceTypes)
	assert.Equal(t, []string{models.ResourceOther}, filters[4].Options.ResourceTypes)
}
