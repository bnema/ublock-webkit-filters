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
	Converted   int
	Skipped     int
	SkipReasons map[string]int
}

// Skip reason constants
const (
	SkipInvalidRegex      = "invalid-regex"
	SkipCosmeticException = "cosmetic-exception"
	SkipEmptySelector     = "empty-selector"
)

// New creates a new converter
func New() *Converter {
	return &Converter{
		stats: Stats{
			SkipReasons: make(map[string]int),
		},
	}
}

// skip records a skipped filter with reason
func (c *Converter) skip(reason string) {
	c.stats.Skipped++
	c.stats.SkipReasons[reason]++
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
		var skipReason string

		switch f.Type {
		case models.FilterTypeNetwork:
			rule, skipReason = c.convertNetwork(f, false)
		case models.FilterTypeException:
			rule, skipReason = c.convertNetwork(f, true)
		case models.FilterTypeCosmetic:
			rule, skipReason = c.convertCosmetic(f, false)
		case models.FilterTypeCosmeticException:
			rule, skipReason = c.convertCosmetic(f, true)
		default:
			continue
		}

		if rule == nil {
			if skipReason != "" {
				c.skip(skipReason)
			}
			continue
		}

		c.stats.Converted++
		rules = append(rules, *rule)
	}

	return rules
}

// convertNetwork converts a network filter to a WebKit rule
func (c *Converter) convertNetwork(f models.Filter, isException bool) (*models.WebKitRule, string) {
	regex := PatternToRegex(f.Pattern)

	// Validate the regex is WebKit-compatible
	if !ValidateRegex(regex) {
		return nil, SkipInvalidRegex
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

	return rule, ""
}

// convertCosmetic converts a cosmetic filter to a WebKit rule
func (c *Converter) convertCosmetic(f models.Filter, isException bool) (*models.WebKitRule, string) {
	if f.Selector == "" {
		return nil, SkipEmptySelector
	}

	// Exception cosmetic filters - WebKit doesn't have a direct equivalent
	if isException {
		return nil, SkipCosmeticException
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

	return rule, ""
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
