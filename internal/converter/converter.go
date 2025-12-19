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
		var convertedRules []models.WebKitRule
		var skipReason string

		switch f.Type {
		case models.FilterTypeNetwork:
			convertedRules, skipReason = c.convertNetwork(f, false)
		case models.FilterTypeException:
			convertedRules, skipReason = c.convertNetwork(f, true)
		case models.FilterTypeCosmetic:
			convertedRules, skipReason = c.convertCosmetic(f, false)
		case models.FilterTypeCosmeticException:
			convertedRules, skipReason = c.convertCosmetic(f, true)
		default:
			continue
		}

		if len(convertedRules) == 0 {
			if skipReason != "" {
				c.skip(skipReason)
			}
			continue
		}

		c.stats.Converted += len(convertedRules)
		rules = append(rules, convertedRules...)
	}

	return rules
}

// convertNetwork converts a network filter to WebKit rules
// Returns multiple rules if splitting is needed (e.g., both if-domain and unless-domain,
// or patterns ending with ^ separator which need both separator-char and end-of-string variants)
func (c *Converter) convertNetwork(f models.Filter, isException bool) ([]models.WebKitRule, string) {
	regex := PatternToRegex(f.Pattern)

	// Validate the regex is WebKit-compatible
	if !ValidateRegex(regex) {
		return nil, SkipInvalidRegex
	}

	// Check if we need an end-anchor variant (pattern ends with ^ separator)
	needsEndAnchorVariant := PatternEndsWithSeparator(f.Pattern)
	var endAnchorRegex string
	if needsEndAnchorVariant {
		endAnchorRegex = PatternToRegexEndAnchor(f.Pattern)
		if !ValidateRegex(endAnchorRegex) {
			// If end-anchor variant is invalid, just use the separator version
			needsEndAnchorVariant = false
		}
	}

	// Determine action type
	actionType := models.ActionBlock
	if isException {
		actionType = models.ActionIgnorePreviousRule
	}

	// Build base trigger options
	var caseSensitive *bool
	if f.Options.MatchCase {
		t := true
		caseSensitive = &t
	}

	var resourceType []string
	if len(f.Options.ResourceTypes) > 0 {
		resourceType = f.Options.ResourceTypes
	}

	var loadType []string
	if f.Options.ThirdParty != nil {
		if *f.Options.ThirdParty {
			loadType = []string{models.LoadThirdParty}
		} else {
			loadType = []string{models.LoadFirstParty}
		}
	}

	hasDomains := len(f.Options.Domains) > 0
	hasExcludeDomains := len(f.Options.ExcludeDomains) > 0

	// WebKit only allows ONE of: if-domain, unless-domain, if-top-url, unless-top-url
	// If both domain types are present, we need to split into separate rules
	if hasDomains && hasExcludeDomains {
		var rules []models.WebKitRule

		// Rule 1: Apply to included domains only (separator char variant)
		rule1 := models.WebKitRule{
			Trigger: models.WebKitTrigger{
				URLFilter:                regex,
				URLFilterIsCaseSensitive: caseSensitive,
				ResourceType:             resourceType,
				LoadType:                 loadType,
				IfDomain:                 normalizeDomains(f.Options.Domains),
			},
			Action: models.WebKitAction{Type: actionType},
		}
		rules = append(rules, rule1)

		// Rule 1b: End-anchor variant for included domains
		if needsEndAnchorVariant {
			rule1b := models.WebKitRule{
				Trigger: models.WebKitTrigger{
					URLFilter:                endAnchorRegex,
					URLFilterIsCaseSensitive: caseSensitive,
					ResourceType:             resourceType,
					LoadType:                 loadType,
					IfDomain:                 normalizeDomains(f.Options.Domains),
				},
				Action: models.WebKitAction{Type: actionType},
			}
			rules = append(rules, rule1b)
		}

		// Rule 2: Apply everywhere except excluded domains (separator char variant)
		rule2 := models.WebKitRule{
			Trigger: models.WebKitTrigger{
				URLFilter:                regex,
				URLFilterIsCaseSensitive: caseSensitive,
				ResourceType:             resourceType,
				LoadType:                 loadType,
				UnlessDomain:             normalizeDomains(f.Options.ExcludeDomains),
			},
			Action: models.WebKitAction{Type: actionType},
		}
		rules = append(rules, rule2)

		// Rule 2b: End-anchor variant for excluded domains
		if needsEndAnchorVariant {
			rule2b := models.WebKitRule{
				Trigger: models.WebKitTrigger{
					URLFilter:                endAnchorRegex,
					URLFilterIsCaseSensitive: caseSensitive,
					ResourceType:             resourceType,
					LoadType:                 loadType,
					UnlessDomain:             normalizeDomains(f.Options.ExcludeDomains),
				},
				Action: models.WebKitAction{Type: actionType},
			}
			rules = append(rules, rule2b)
		}

		return rules, ""
	}

	// Single rule case - only one or no domain condition
	var rules []models.WebKitRule

	// Main rule with separator char matching
	rule := models.WebKitRule{
		Trigger: models.WebKitTrigger{
			URLFilter:                regex,
			URLFilterIsCaseSensitive: caseSensitive,
			ResourceType:             resourceType,
			LoadType:                 loadType,
		},
		Action: models.WebKitAction{Type: actionType},
	}

	if hasDomains {
		rule.Trigger.IfDomain = normalizeDomains(f.Options.Domains)
	}
	if hasExcludeDomains {
		rule.Trigger.UnlessDomain = normalizeDomains(f.Options.ExcludeDomains)
	}

	rules = append(rules, rule)

	// Add end-anchor variant if pattern ends with separator
	if needsEndAnchorVariant {
		endRule := models.WebKitRule{
			Trigger: models.WebKitTrigger{
				URLFilter:                endAnchorRegex,
				URLFilterIsCaseSensitive: caseSensitive,
				ResourceType:             resourceType,
				LoadType:                 loadType,
			},
			Action: models.WebKitAction{Type: actionType},
		}

		if hasDomains {
			endRule.Trigger.IfDomain = normalizeDomains(f.Options.Domains)
		}
		if hasExcludeDomains {
			endRule.Trigger.UnlessDomain = normalizeDomains(f.Options.ExcludeDomains)
		}

		rules = append(rules, endRule)
	}

	return rules, ""
}

// convertCosmetic converts a cosmetic filter to WebKit rules
// Returns multiple rules if splitting is needed (e.g., both if-domain and unless-domain)
func (c *Converter) convertCosmetic(f models.Filter, isException bool) ([]models.WebKitRule, string) {
	if f.Selector == "" {
		return nil, SkipEmptySelector
	}

	// Exception cosmetic filters - WebKit doesn't have a direct equivalent
	if isException {
		return nil, SkipCosmeticException
	}

	// Parse domains into include/exclude lists
	var include, exclude []string
	for _, d := range f.Domains {
		if strings.HasPrefix(d, "~") {
			exclude = append(exclude, normalizeDomain(d[1:]))
		} else {
			include = append(include, normalizeDomain(d))
		}
	}

	hasInclude := len(include) > 0
	hasExclude := len(exclude) > 0

	// WebKit only allows ONE of: if-domain, unless-domain, if-top-url, unless-top-url
	// If both domain types are present, we need to split into separate rules
	if hasInclude && hasExclude {
		var rules []models.WebKitRule

		// Rule 1: Apply to included domains only
		rule1 := models.WebKitRule{
			Trigger: models.WebKitTrigger{
				URLFilter: ".*",
				IfDomain:  include,
			},
			Action: models.WebKitAction{
				Type:     models.ActionCSSDisplayNone,
				Selector: f.Selector,
			},
		}
		rules = append(rules, rule1)

		// Rule 2: Apply everywhere except excluded domains
		rule2 := models.WebKitRule{
			Trigger: models.WebKitTrigger{
				URLFilter:    ".*",
				UnlessDomain: exclude,
			},
			Action: models.WebKitAction{
				Type:     models.ActionCSSDisplayNone,
				Selector: f.Selector,
			},
		}
		rules = append(rules, rule2)

		return rules, ""
	}

	// Single rule case
	rule := models.WebKitRule{
		Trigger: models.WebKitTrigger{
			URLFilter: ".*",
		},
		Action: models.WebKitAction{
			Type:     models.ActionCSSDisplayNone,
			Selector: f.Selector,
		},
	}

	if hasInclude {
		rule.Trigger.IfDomain = include
	}
	if hasExclude {
		rule.Trigger.UnlessDomain = exclude
	}

	return []models.WebKitRule{rule}, ""
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
