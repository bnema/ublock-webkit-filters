package converter

// WebKit Content Blocker Regex Constraints
//
// WebKit's Content Blocker uses a strict subset of JavaScript regular expressions.
// Many common regex features are NOT supported due to performance constraints.
//
// References:
// - https://webkit.org/blog/3476/content-blockers-first-look/
// - https://github.com/AduardTeam/SafariConverterLib
//
// SUPPORTED FEATURES:
// - . (dot)           - Match any single character
// - [a-z]             - Character ranges/classes
// - [^a-z]            - Negated character classes
// - ()                - Grouping (non-capturing only)
// - *                 - Zero or more (greedy)
// - +                 - One or more (greedy)
// - ?                 - Zero or one (greedy)
// - ^                 - Start anchor (ONLY at beginning of pattern)
// - $                 - End anchor (ONLY at end of pattern)
// - \. \/ \\ etc      - Escaped literal characters
//
// UNSUPPORTED FEATURES:
// - \w \W             - Word character class (use [a-zA-Z0-9_] instead)
// - \d \D             - Digit character class (use [0-9] instead)
// - \s \S             - Whitespace character class (use [ \t\n\r\f\v] instead)
// - \b \B             - Word boundary
// - {n} {n,} {n,m}    - Numeric quantifiers (must expand or skip)
// - |                 - Alternation/disjunction outside character classes
// - (?=...)           - Positive lookahead
// - (?!...)           - Negative lookahead
// - (?<=...)          - Positive lookbehind
// - (?<!...)          - Negative lookbehind
// - (?:...)           - Non-capturing groups (may work but not guaranteed)
// - (?P<name>...)     - Named groups
// - \p{...} \P{...}   - Unicode property escapes
// - Non-ASCII chars   - Unicode characters in patterns
//
// PERFORMANCE NOTES:
// - Safari compiles rules into finite state machines
// - Complex patterns significantly increase compile time
// - Use url-filter-is-case-sensitive when possible
// - Avoid patterns like foo.*bar (quantifier in middle)
// - Maximum 50,000 rules per content blocker

import (
	"strings"
)

// Note: reNumericQuantifier, reNonASCII, and reWordBoundary are defined in regex.go

// Patterns for detecting unsupported regex features (additional)
var (
// Shorthand character classes (for reporting - actual expansion is in regex.go)
// reShorthandClasses matches \w, \W, \d, \D, \s, \S, \b, \B
)

// WebKitRegexIssue describes a problem found in a regex pattern
type WebKitRegexIssue struct {
	Pattern     string
	Issue       string
	Fixable     bool
	Replacement string
}

// CheckWebKitCompatibility analyzes a regex pattern for WebKit compatibility issues
func CheckWebKitCompatibility(pattern string) []WebKitRegexIssue {
	var issues []WebKitRegexIssue

	// Check for shorthand character classes (\w, \d, \s, etc.)
	shorthandPatterns := []struct {
		match       string
		replacement string
		fixable     bool
	}{
		{`\w`, `[a-zA-Z0-9_]`, true},
		{`\W`, `[^a-zA-Z0-9_]`, true},
		{`\d`, `[0-9]`, true},
		{`\D`, `[^0-9]`, true},
		{`\s`, `[ \t\n\r\f\v]`, true},
		{`\S`, `[^ \t\n\r\f\v]`, true},
		{`\b`, "", false},
		{`\B`, "", false},
	}

	for _, sp := range shorthandPatterns {
		if strings.Contains(pattern, sp.match) {
			issues = append(issues, WebKitRegexIssue{
				Pattern:     pattern,
				Issue:       "shorthand character class: " + sp.match,
				Fixable:     sp.fixable,
				Replacement: sp.replacement,
			})
		}
	}

	// Check for numeric quantifiers {n}, {n,}, {n,m}
	// Note: {n,} is fixable as it's converted to + in expandCharacterClasses
	if matches := reNumericQuantifierOpen.FindAllString(pattern, -1); len(matches) > 0 {
		for _, m := range matches {
			issues = append(issues, WebKitRegexIssue{
				Pattern:     pattern,
				Issue:       "numeric quantifier: " + m,
				Fixable:     true,
				Replacement: "+",
			})
		}
	}
	if matches := reNumericQuantifier.FindAllString(pattern, -1); len(matches) > 0 {
		for _, m := range matches {
			issues = append(issues, WebKitRegexIssue{
				Pattern: pattern,
				Issue:   "numeric quantifier: " + m,
				Fixable: false,
			})
		}
	}

	// Check for disjunctions (| outside character classes)
	if containsDisjunction(pattern) {
		issues = append(issues, WebKitRegexIssue{
			Pattern: pattern,
			Issue:   "disjunction (|) outside character class",
			Fixable: false,
		})
	}

	// Check for non-ASCII characters
	if reNonASCII.MatchString(pattern) {
		issues = append(issues, WebKitRegexIssue{
			Pattern: pattern,
			Issue:   "non-ASCII characters",
			Fixable: false,
		})
	}

	// Check for unsupported assertions
	unsupportedAssertions := []struct {
		pattern string
		name    string
	}{
		{`(?<!`, "negative lookbehind"},
		{`(?<=`, "positive lookbehind"},
		{`(?=`, "positive lookahead"},
		{`(?!`, "negative lookahead"},
		{`(?P<`, "named group"},
		{`(?<`, "named group"},
		{`\p{`, "unicode property"},
		{`\P{`, "unicode property"},
	}

	for _, ua := range unsupportedAssertions {
		if strings.Contains(pattern, ua.pattern) {
			issues = append(issues, WebKitRegexIssue{
				Pattern: pattern,
				Issue:   ua.name,
				Fixable: false,
			})
		}
	}

	return issues
}

// HasUnfixableIssues returns true if the pattern has issues that cannot be fixed
func HasUnfixableIssues(pattern string) bool {
	issues := CheckWebKitCompatibility(pattern)
	for _, issue := range issues {
		if !issue.Fixable {
			return true
		}
	}
	return false
}

// DescribeIssues returns a human-readable description of all issues
func DescribeIssues(issues []WebKitRegexIssue) string {
	if len(issues) == 0 {
		return ""
	}
	var parts []string
	for _, issue := range issues {
		parts = append(parts, issue.Issue)
	}
	return strings.Join(parts, ", ")
}
