package converter

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/bnema/ublock-webkit-filters/internal/models"
	"github.com/bnema/ublock-webkit-filters/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParserToConverterFlow(t *testing.T) {
	tests := []struct {
		name           string
		filterLine     string
		expectSkipped  bool
		expectExpanded bool // if not skipped, check if \w was expanded
	}{
		{
			name:           "regex with \\w and numeric quantifier should be converted (approximated)",
			filterLine:     `/(https?:\/\/)\w{30,}\.me\/\w{30,}\./$script,third-party`,
			expectSkipped:  false,
			expectExpanded: true,
		},
		{
			name:          "regex with exact numeric quantifier should be skipped",
			filterLine:    `/(https?:\/\/)\w{30}\.me\/\w{30}\./$script`,
			expectSkipped: true,
		},
		{
			name:           "regex with \\w and basic quantifier should be converted",
			filterLine:     `/(https?:\/\/)\w+\.me\/\w+\./$script,third-party`,
			expectSkipped:  false,
			expectExpanded: true,
		},
		{
			name:          "regex with disjunction should be skipped",
			filterLine:    `/foo|bar/$script`,
			expectSkipped: true,
		},
		{
			name:           "simple network filter should be converted",
			filterLine:     `||example.com^`,
			expectSkipped:  false,
			expectExpanded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the filter
			p := parser.New()
			filters, err := p.Parse(strings.NewReader(tt.filterLine))
			assert.NoError(t, err)
			assert.Len(t, filters, 1)

			// Convert
			c := New()
			rules := c.Convert(filters)

			if tt.expectSkipped {
				assert.Empty(t, rules, "Expected rule to be skipped")
				assert.Equal(t, 1, c.stats.Skipped, "Expected skip count to be 1")
			} else {
				assert.NotEmpty(t, rules, "Expected rule to be converted")

				if tt.expectExpanded {
					// Check that \w was expanded in the url-filter
					urlFilter := rules[0].Trigger.URLFilter
					assert.NotContains(t, urlFilter, `\w`, "Expected \\w to be expanded")
					assert.Contains(t, urlFilter, `[a-zA-Z0-9_]`, "Expected expanded character class")
				}
			}
		})
	}
}

func TestPatternToRegexWithExpansion(t *testing.T) {
	// Test that regex patterns get character classes expanded
	input := `/(https?:\/\/)\w+\.me\/\w+\./`
	result := PatternToRegex(input)

	assert.NotContains(t, result, `\w`)
	assert.Contains(t, result, `[a-zA-Z0-9_]`)

	// And it should pass validation (no numeric quantifiers)
	assert.True(t, ValidateRegex(result), "Expected expanded regex to be valid")
}

func TestPatternWithNumericQuantifierSucceedsByApproximation(t *testing.T) {
	// Test that numeric quantifiers {n,} now succeed by being converted to +
	input := `/(https?:\/\/)\w{30,}\.me\/\w{30,}\./`
	result := PatternToRegex(input)

	// The \w should be expanded and {30,} should become +
	assert.Contains(t, result, `[a-zA-Z0-9_]+`)
	assert.NotContains(t, result, `{30,}`)

	// And validation should succeed
	assert.True(t, ValidateRegex(result), "Expected regex with approximated numeric quantifier to pass validation")
}

func TestPatternWithFixedNumericQuantifierFails(t *testing.T) {
	// Test that exact numeric quantifiers still fail
	input := `/(https?:\/\/)\w{30}\.me\/\w{30}\./`
	result := PatternToRegex(input)

	assert.False(t, ValidateRegex(result), "Expected regex with fixed numeric quantifier to fail validation")
}

func TestDirectConversionWithRegex(t *testing.T) {
	// Create a filter directly with {n,} quantifier
	f := models.Filter{
		Type:    models.FilterTypeNetwork,
		Pattern: `/(https?:\/\/)\w{30,}\.me\/\w{30,}\./`,
		Options: models.FilterOptions{
			ResourceTypes: []string{models.ResourceScript},
		},
	}

	c := New()
	rules := c.Convert([]models.Filter{f})

	// Should NOT be skipped anymore, but converted
	assert.NotEmpty(t, rules, "Expected rule with open numeric quantifier to be converted")
	assert.Equal(t, 0, c.stats.Skipped)
}

func TestRegressionPingitFilter(t *testing.T) {
	// Real filter that caused GitHub/Google Maps to break
	p := parser.New()
	filters, err := p.Parse(strings.NewReader(
		`*$script,3p,denyallow=cdn77.org|google.com|gstatic.com|recaptcha.net|smartsuppcdn.com|smartsuppchat.com,domain=pingit.*|~pingit.com`,
	))
	assert.NoError(t, err)
	// Must be skipped: has denyallow=
	assert.Empty(t, filters, "pingit filter with denyallow= must be skipped at parse time")
}

func TestRegressionMylinkFilter(t *testing.T) {
	p := parser.New()
	filters, err := p.Parse(strings.NewReader(
		`*$script,3p,denyallow=cloudflare.com|cloudflare.net|consensu.org|google.com|googleapis.com|gstatic.com|hcaptcha.com|hwcdn.net|recaptcha.net|twitter.com|x.com,domain=mylink.*|my1ink.*|myl1nk.*|myli3k.*|~mylink.tel`,
	))
	assert.NoError(t, err)
	assert.Empty(t, filters, "mylink filter with denyallow= must be skipped at parse time")
}

func TestRegressionPussyspaceFilter(t *testing.T) {
	// This filter uses both from= and denyallow= — denyallow= causes skip
	p := parser.New()
	filters, err := p.Parse(strings.NewReader(
		`$image,3p,denyallow=cdn77.org|fpbns.net|globalcdn.co,from=pussyspace.com|pussyspace.net`,
	))
	assert.NoError(t, err)
	assert.Empty(t, filters, "pussyspace filter with denyallow= must be skipped at parse time")
}

func TestRegressionScnlogFilter(t *testing.T) {
	p := parser.New()
	filters, err := p.Parse(strings.NewReader(
		`*$script,3p,denyallow=cloudflare.com|cloudflare.net|google.com|googleapis.com|gstatic.com|jwpcdn.com,from=scnlog.me`,
	))
	assert.NoError(t, err)
	assert.Empty(t, filters, "scnlog filter with denyallow= must be skipped at parse time")
}

func TestRegressionEntityDomainWithoutDenyallow(t *testing.T) {
	// Exercises converter-level skip (entity domain + include/exclude)
	// when denyallow= is NOT present — this is the safety net path
	p := parser.New()
	filters, err := p.Parse(strings.NewReader(
		`*$script,3p,domain=pingit.*|~pingit.com`,
	))
	assert.NoError(t, err)
	assert.Len(t, filters, 1, "parser should pass this through (no denyallow=)")

	c := New()
	rules := c.Convert(filters)
	assert.Empty(t, rules, "entity domain + include/exclude filter must be skipped at converter level")
}

func TestNoCatchAllBlockRulesInOutput(t *testing.T) {
	// Simulate a mix of safe and dangerous filters
	input := `||ads.example.com^
*$script,3p,denyallow=google.com,domain=pingit.*|~pingit.com
*$doc,ipaddress=178.16.53.131
*$strict3p,ipaddress=0.0.0.0,domain=~0.0.0.0|~127.0.0.1|~[::1]|~[::]|~local|~localhost
||tracking.example.com^$third-party
*$image,3p,denyallow=cdn77.org,from=pussyspace.com
||safe-ads.example.com^$image
`
	p := parser.New()
	filters, err := p.Parse(strings.NewReader(input))
	assert.NoError(t, err)

	c := New()
	rules := c.Convert(filters)
	rules = Deduplicate(rules)

	// Verify no unsafe catch-all rules exist.
	// A broad rule is only acceptable when WebKit has a precise resource type
	// for it; top-level documents and raw buckets are too risky here.
	for _, r := range rules {
		if r.Trigger.URLFilter == ".*" && r.Action.Type == models.ActionBlock {
			if len(r.Trigger.IfDomain) == 0 &&
				len(r.Trigger.UnlessDomain) == 0 &&
				(containsString(r.Trigger.ResourceType, models.ResourceTopDocument) ||
					containsString(r.Trigger.ResourceType, models.ResourceDocument) ||
					containsString(r.Trigger.ResourceType, models.ResourceRaw)) {
				t.Errorf("Found catch-all block rule with no domain restriction: %+v", r)
			}
		}
	}

	// Safe filters should still be converted
	assert.True(t, len(rules) >= 2, "Safe filters should produce rules, got %d", len(rules))
}

func TestUnsupportedWildcardModifiersDoNotBlockRealSites(t *testing.T) {
	// Before the parser fix, the first two lines below degraded into global
	// ".*" block rules and would have blocked any top-level document, including
	// reddit.com and lesnumeriques.com.
	input := `*$doc,ipaddress=178.16.53.131
*$strict3p,ipaddress=0.0.0.0,domain=~0.0.0.0|~127.0.0.1|~[::1]|~[::]|~local|~localhost
*$document,domain=malware.example
`

	p := parser.New()
	filters, err := p.Parse(strings.NewReader(input))
	require.NoError(t, err)
	assert.Len(t, filters, 1, "only the representable scoped document rule should survive parsing")
	assert.Equal(t, 2, p.Stats().Unsupported, "the unsupported wildcard modifiers must be rejected")

	c := New()
	rules := Deduplicate(c.Convert(filters))

	assert.False(t, blocksTopLevelDocument(t, rules, "https://www.reddit.com/"))
	assert.False(t, blocksTopLevelDocument(t, rules, "https://www.lesnumeriques.com/"))
	assert.True(t, blocksTopLevelDocument(t, rules, "https://malware.example/"))
	assert.True(t, blocksTopLevelDocument(t, rules, "https://sub.malware.example/news"))
}

func TestPingRuleUsesPreciseWebKitPingType(t *testing.T) {
	input := `$ping,third-party
*$document,domain=malware.example
`

	p := parser.New()
	filters, err := p.Parse(strings.NewReader(input))
	require.NoError(t, err)
	assert.Len(t, filters, 2)
	assert.Zero(t, p.Stats().Unsupported)

	c := New()
	rules := Deduplicate(c.Convert(filters))

	var sawPrecisePingRule bool
	for _, r := range rules {
		if r.Trigger.URLFilter == ".*" &&
			r.Action.Type == models.ActionBlock &&
			containsString(r.Trigger.ResourceType, models.ResourcePing) &&
			containsString(r.Trigger.LoadType, models.LoadThirdParty) {
			sawPrecisePingRule = true
		}

		assert.False(t,
			r.Trigger.URLFilter == ".*" &&
				r.Action.Type == models.ActionBlock &&
				containsString(r.Trigger.ResourceType, models.ResourceRaw) &&
				containsString(r.Trigger.LoadType, models.LoadThirdParty) &&
				len(r.Trigger.IfDomain) == 0 &&
				len(r.Trigger.UnlessDomain) == 0,
			"ping conversion must not create a global raw third-party block: %+v", r)
	}

	assert.True(t, sawPrecisePingRule, "expected a ping-specific third-party block rule")
	assert.False(t, blocksTopLevelDocument(t, rules, "https://www.reddit.com/"))
	assert.True(t, blocksTopLevelDocument(t, rules, "https://malware.example/"))
}

func TestPreciseWebKitResourceTypeConversion(t *testing.T) {
	input := `||example.com^$doc
||example.com^$subdocument
||example.com^$xhr
||example.com^$websocket
||example.com^$other
`

	p := parser.New()
	filters, err := p.Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, filters, 5)

	c := New()
	rules := c.Convert(filters)
	require.Len(t, rules, 10, "each separator-anchored rule gets a second end-anchor variant")

	assert.Contains(t, rules[0].Trigger.ResourceType, models.ResourceTopDocument)
	assert.Contains(t, rules[2].Trigger.ResourceType, models.ResourceChildDocument)
	assert.Contains(t, rules[4].Trigger.ResourceType, models.ResourceFetch)
	assert.Contains(t, rules[6].Trigger.ResourceType, models.ResourceWebSocket)
	assert.Contains(t, rules[8].Trigger.ResourceType, models.ResourceOther)
}

func blocksTopLevelDocument(t *testing.T, rules []models.WebKitRule, rawURL string) bool {
	t.Helper()

	u, err := url.Parse(rawURL)
	require.NoError(t, err)

	blocked := false
	for _, rule := range rules {
		if !matchesTopLevelDocumentRule(t, rule, u) {
			continue
		}

		switch rule.Action.Type {
		case models.ActionBlock:
			blocked = true
		case models.ActionIgnorePreviousRule:
			blocked = false
		}
	}

	return blocked
}

func matchesTopLevelDocumentRule(t *testing.T, rule models.WebKitRule, u *url.URL) bool {
	t.Helper()

	if len(rule.Trigger.ResourceType) > 0 &&
		!containsString(rule.Trigger.ResourceType, models.ResourceTopDocument) &&
		!containsString(rule.Trigger.ResourceType, models.ResourceDocument) {
		return false
	}

	// Treat top-level navigations as first-party. This is enough for the
	// regression cases we care about here.
	if len(rule.Trigger.LoadType) > 0 && !containsString(rule.Trigger.LoadType, models.LoadFirstParty) {
		return false
	}

	pattern := rule.Trigger.URLFilter
	caseSensitive := rule.Trigger.URLFilterIsCaseSensitive != nil && *rule.Trigger.URLFilterIsCaseSensitive
	if !caseSensitive {
		pattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(pattern)
	require.NoError(t, err)
	if !re.MatchString(u.String()) {
		return false
	}

	host := strings.ToLower(u.Hostname())
	if len(rule.Trigger.IfDomain) > 0 && !matchesAnyDomain(host, rule.Trigger.IfDomain) {
		return false
	}
	if len(rule.Trigger.UnlessDomain) > 0 && matchesAnyDomain(host, rule.Trigger.UnlessDomain) {
		return false
	}

	return true
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func matchesAnyDomain(host string, domains []string) bool {
	for _, domain := range domains {
		if matchesDomain(host, domain) {
			return true
		}
	}
	return false
}

func matchesDomain(host, pattern string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))

	switch {
	case pattern == "":
		return false
	case strings.HasPrefix(pattern, "*"):
		base := strings.TrimPrefix(pattern, "*")
		base = strings.TrimPrefix(base, ".")
		return host == base || strings.HasSuffix(host, "."+base)
	case strings.HasPrefix(pattern, "."):
		base := strings.TrimPrefix(pattern, ".")
		return strings.HasSuffix(host, "."+base)
	default:
		return host == pattern
	}
}
