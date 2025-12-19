package converter

import (
	"strings"

	"github.com/bnema/ublock-webkit-filters/internal/models"
)

// Converter converts parsed filters to WebKit rules
type Converter struct {
	stats Stats
}

// Stats tracks conversion statistics
type Stats struct {
	Converted int
	Skipped   int
}

// New creates a new converter
func New() *Converter {
	return &Converter{}
}

// Stats returns conversion statistics
func (c *Converter) Stats() Stats {
	return c.stats
}

// Convert transforms parsed filters into WebKit rules
func (c *Converter) Convert(filters []models.Filter) []models.WebKitRule {
	var rules []models.WebKitRule

	for _, f := range filters {
		var rule *models.WebKitRule
		var err error

		switch f.Type {
		case models.FilterTypeNetwork:
			rule, err = c.convertNetwork(f, false)
		case models.FilterTypeException:
			rule, err = c.convertNetwork(f, true)
		case models.FilterTypeCosmetic:
			rule, err = c.convertCosmetic(f, false)
		case models.FilterTypeCosmeticException:
			rule, err = c.convertCosmetic(f, true)
		default:
			c.stats.Skipped++
			continue
		}

		if err != nil || rule == nil {
			c.stats.Skipped++
			continue
		}

		c.stats.Converted++
		rules = append(rules, *rule)
	}

	return rules
}

// convertNetwork converts a network filter to a WebKit rule
func (c *Converter) convertNetwork(f models.Filter, isException bool) (*models.WebKitRule, error) {
	regex := PatternToRegex(f.Pattern)

	// Validate the regex is WebKit-compatible
	if !ValidateRegex(regex) {
		return nil, nil
	}

	rule := &models.WebKitRule{
		Trigger: models.WebKitTrigger{
			URLFilter: regex,
		},
		Action: models.WebKitAction{
			Type: models.ActionBlock,
		},
	}

	// Exception rules use ignore-previous-rules
	if isException {
		rule.Action.Type = models.ActionIgnorePreviousRule
	}

	// Apply options
	if f.Options.MatchCase {
		t := true
		rule.Trigger.URLFilterIsCaseSensitive = &t
	}

	// Resource types
	if len(f.Options.ResourceTypes) > 0 {
		rule.Trigger.ResourceType = f.Options.ResourceTypes
	}

	// Load type (first/third party)
	if f.Options.ThirdParty != nil {
		if *f.Options.ThirdParty {
			rule.Trigger.LoadType = []string{models.LoadThirdParty}
		} else {
			rule.Trigger.LoadType = []string{models.LoadFirstParty}
		}
	}

	// Domain restrictions
	if len(f.Options.Domains) > 0 {
		rule.Trigger.IfDomain = normalizeDomains(f.Options.Domains)
	}
	if len(f.Options.ExcludeDomains) > 0 {
		rule.Trigger.UnlessDomain = normalizeDomains(f.Options.ExcludeDomains)
	}

	return rule, nil
}

// convertCosmetic converts a cosmetic filter to a WebKit rule
func (c *Converter) convertCosmetic(f models.Filter, isException bool) (*models.WebKitRule, error) {
	if f.Selector == "" {
		return nil, nil
	}

	// Exception cosmetic filters - use ignore-previous-rules
	if isException {
		// Cosmetic exceptions are tricky - we'll skip them for now
		// as WebKit doesn't have a direct equivalent
		return nil, nil
	}

	rule := &models.WebKitRule{
		Trigger: models.WebKitTrigger{
			URLFilter: ".*",
		},
		Action: models.WebKitAction{
			Type:     models.ActionCSSDisplayNone,
			Selector: f.Selector,
		},
	}

	// Domain-specific cosmetic filters
	if len(f.Domains) > 0 {
		var include, exclude []string
		for _, d := range f.Domains {
			if strings.HasPrefix(d, "~") {
				exclude = append(exclude, normalizeDomain(d[1:]))
			} else {
				include = append(include, normalizeDomain(d))
			}
		}
		if len(include) > 0 {
			rule.Trigger.IfDomain = include
		}
		if len(exclude) > 0 {
			rule.Trigger.UnlessDomain = exclude
		}
	}

	return rule, nil
}

// normalizeDomains adds * prefix for wildcard matching
func normalizeDomains(domains []string) []string {
	result := make([]string, len(domains))
	for i, d := range domains {
		result[i] = normalizeDomain(d)
	}
	return result
}

// normalizeDomain ensures domain has proper format for WebKit
func normalizeDomain(d string) string {
	d = strings.ToLower(strings.TrimSpace(d))
	// WebKit expects domains with * prefix for subdomains
	if !strings.HasPrefix(d, "*") && !strings.HasPrefix(d, ".") {
		return "*" + d
	}
	return d
}
