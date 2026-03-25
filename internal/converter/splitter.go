package converter

import (
	"fmt"
	"sort"

	"github.com/bnema/ublock-webkit-filters/internal/models"
)

// MaxRulesPerFile is Safari/WebKit's limit per content blocker
const MaxRulesPerFile = 150000

// Splitter splits rules into chunks respecting the 150k limit
type Splitter struct {
	maxRules int
}

// NewSplitter creates a splitter with the given max rules per file
func NewSplitter(maxRules int) *Splitter {
	if maxRules <= 0 {
		maxRules = MaxRulesPerFile
	}
	return &Splitter{maxRules: maxRules}
}

// Split divides rules into multiple files if needed
// Returns a map of filename suffix -> rules
func (s *Splitter) Split(rules []models.WebKitRule, baseName string) map[string][]models.WebKitRule {
	result := make(map[string][]models.WebKitRule)

	if len(rules) <= s.maxRules {
		result[baseName] = rules
		return result
	}

	numParts := (len(rules) + s.maxRules - 1) / s.maxRules

	for i := 0; i < numParts; i++ {
		start := i * s.maxRules
		end := start + s.maxRules
		if end > len(rules) {
			end = len(rules)
		}

		filename := fmt.Sprintf("%s-part%d", baseName, i+1)
		result[filename] = rules[start:end]
	}

	return result
}

// sortedKey returns a sorted copy of a string slice for canonical key building
func sortedKey(s []string) []string {
	if len(s) == 0 {
		return s
	}
	c := make([]string, len(s))
	copy(c, s)
	sort.Strings(c)
	return c
}

// Deduplicate removes duplicate rules based on their complete trigger+action
func Deduplicate(rules []models.WebKitRule) []models.WebKitRule {
	seen := make(map[string]bool)
	result := make([]models.WebKitRule, 0, len(rules))

	for _, r := range rules {
		// Build canonical key from ALL trigger and action fields
		caseSensitive := false
		if r.Trigger.URLFilterIsCaseSensitive != nil {
			caseSensitive = *r.Trigger.URLFilterIsCaseSensitive
		}

		key := fmt.Sprintf("%s|%s|%s|%v|%v|%v|%v|%v",
			r.Trigger.URLFilter,
			r.Action.Type,
			r.Action.Selector,
			sortedKey(r.Trigger.IfDomain),
			sortedKey(r.Trigger.UnlessDomain),
			sortedKey(r.Trigger.ResourceType),
			sortedKey(r.Trigger.LoadType),
			caseSensitive,
		)

		if !seen[key] {
			seen[key] = true
			result = append(result, r)
		}
	}

	return result
}
