package converter

import (
	"regexp"
	"strings"
)

// Regex patterns ported from uBlock's make-rulesets.js:196-234
const (
	// Separator matches any non-alphanumeric character (WebKit doesn't support disjunctions)
	restrSeparator = `[^%.0-9a-z_-]`
	// Hostname anchor for patterns starting with ||
	restrHostnameAnchor1 = `^[a-z-]+://(?:[^/?#]+\.)?`
	// Hostname anchor for patterns starting with ||.
	restrHostnameAnchor2 = `^[a-z-]+://(?:[^/?#]+)?`
)

var (
	// Characters to escape in regex (except * and ^)
	rePlainChars = regexp.MustCompile(`[.+?${}()|[\]\\]`)
	// Dangling asterisks at start/end
	reDanglingAsterisks = regexp.MustCompile(`^\*+|\*+$`)
	// Asterisks in pattern
	reAsterisks = regexp.MustCompile(`\*+`)
	// Separator placeholder
	reSeparators = regexp.MustCompile(`\^`)
	// Shorthand character classes (WebKit doesn't support these)
	reWordChar     = regexp.MustCompile(`\\w`)
	reNonWordChar  = regexp.MustCompile(`\\W`)
	reDigitChar    = regexp.MustCompile(`\\d`)
	reNonDigitChar = regexp.MustCompile(`\\D`)
	reSpaceChar    = regexp.MustCompile(`\\s`)
	reNonSpaceChar = regexp.MustCompile(`\\S`)
	// Numeric quantifiers: {n,} - can be approximated with +
	reNumericQuantifierOpen = regexp.MustCompile(`\{[0-9]+,\}`)
)

// PatternToRegex converts an ABP/uBlock pattern to a WebKit-compatible regex
func PatternToRegex(pattern string) string {
	if pattern == "" || pattern == "*" {
		return ".*"
	}

	s := pattern
	anchor := 0 // 0b100 = hostname (||), 0b010 = left (|), 0b001 = right (|)

	// Check for hostname anchor ||
	if strings.HasPrefix(s, "||") {
		anchor = 0b100
		s = s[2:]
	} else if strings.HasPrefix(s, "|") {
		anchor = 0b010
		s = s[1:]
	}

	// Check for right anchor |
	if strings.HasSuffix(s, "|") {
		anchor |= 0b001
		s = s[:len(s)-1]
	}

	// Handle regex patterns (enclosed in /.../)
	if strings.HasPrefix(s, "/") && strings.HasSuffix(s, "/") && len(s) > 2 {
		// It's already a regex, remove the slashes and expand character classes
		regex := s[1 : len(s)-1]
		return expandCharacterClasses(regex)
	}

	// Escape special regex characters (except * and ^)
	reStr := rePlainChars.ReplaceAllString(s, `\$0`)

	// Convert ^ to separator pattern
	reStr = reSeparators.ReplaceAllString(reStr, restrSeparator)

	// Remove dangling asterisks
	reStr = reDanglingAsterisks.ReplaceAllString(reStr, "")

	// Convert * to non-greedy match
	reStr = reAsterisks.ReplaceAllString(reStr, `.*`)

	// Apply anchors
	if anchor&0b100 != 0 {
		// Hostname anchor
		if strings.HasPrefix(reStr, `\.`) {
			reStr = restrHostnameAnchor2 + reStr
		} else {
			reStr = restrHostnameAnchor1 + reStr
		}
	} else if anchor&0b010 != 0 {
		// Left anchor
		reStr = "^" + reStr
	}

	if anchor&0b001 != 0 {
		// Right anchor
		reStr = reStr + "$"
	}

	return reStr
}

// Patterns for detecting unsupported WebKit regex features
var (
	// Numeric quantifiers: {n} or {n,m} - WebKit doesn't support these
	reNumericQuantifier = regexp.MustCompile(`\{[0-9]+(,[0-9]+)?\}`)
	// Non-ASCII characters - WebKit doesn't support these in patterns
	reNonASCII = regexp.MustCompile(`[^\x00-\x7F]`)
	// Word boundary assertions - WebKit doesn't support these
	reWordBoundary = regexp.MustCompile(`\\[bB]`)
)

// ValidateRegex checks if a regex is valid for WebKit
// WebKit has a strict subset of regex features
func ValidateRegex(pattern string) bool {
	// Try to compile the regex
	_, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}

	// WebKit doesn't support these assertion/group patterns
	unsupported := []string{
		`(?<!`, // negative lookbehind
		`(?<=`, // positive lookbehind
		`(?=`,  // positive lookahead
		`(?!`,  // negative lookahead
		`\p{`,  // unicode properties
		`\P{`,  // negated unicode properties
		`(?P<`, // named groups
		`(?<`,  // named groups alternate syntax
	}

	for _, u := range unsupported {
		if strings.Contains(pattern, u) {
			return false
		}
	}

	// Check for disjunctions (| outside character classes)
	if containsDisjunction(pattern) {
		return false
	}

	// Check for numeric quantifiers {n}, {n,}, {n,m}
	if reNumericQuantifier.MatchString(pattern) {
		return false
	}

	// Check for non-ASCII characters
	if reNonASCII.MatchString(pattern) {
		return false
	}

	// Check for word boundary assertions \b, \B
	if reWordBoundary.MatchString(pattern) {
		return false
	}

	// WebKit doesn't support shorthand character classes \w, \d, \s, etc.
	// These should have been expanded by expandCharacterClasses
	if reWordChar.MatchString(pattern) || reNonWordChar.MatchString(pattern) ||
		reDigitChar.MatchString(pattern) || reNonDigitChar.MatchString(pattern) ||
		reSpaceChar.MatchString(pattern) || reNonSpaceChar.MatchString(pattern) {
		return false
	}

	return true
}

// containsDisjunction checks if a regex contains | outside of character classes
func containsDisjunction(pattern string) bool {
	inCharClass := false
	escaped := false

	for _, ch := range pattern {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '[' && !inCharClass {
			inCharClass = true
			continue
		}
		if ch == ']' && inCharClass {
			inCharClass = false
			continue
		}
		if ch == '|' && !inCharClass {
			return true
		}
	}
	return false
}

// PatternEndsWithSeparator checks if the original pattern ends with ^ separator
func PatternEndsWithSeparator(pattern string) bool {
	// Strip right anchor first
	s := strings.TrimSuffix(pattern, "|")
	return strings.HasSuffix(s, "^")
}

// PatternToRegexEndAnchor creates a variant regex with $ end anchor instead of separator
// Used when original pattern ends with ^, to match URLs ending at the pattern
func PatternToRegexEndAnchor(pattern string) string {
	// Strip the trailing ^ and any right anchor
	s := strings.TrimSuffix(pattern, "|")
	s = strings.TrimSuffix(s, "^")

	// Convert without the separator, then add $ end anchor
	regex := PatternToRegex(s)

	// Add end anchor if not already present
	if !strings.HasSuffix(regex, "$") {
		regex = regex + "$"
	}

	return regex
}

// expandCharacterClasses replaces shorthand character classes with explicit equivalents
// WebKit's Content Blocker regex engine doesn't support \w, \d, \s, etc.
func expandCharacterClasses(pattern string) string {
	// Order matters: replace uppercase (negated) first to avoid partial replacements
	pattern = reNonWordChar.ReplaceAllString(pattern, `[^a-zA-Z0-9_]`)
	pattern = reWordChar.ReplaceAllString(pattern, `[a-zA-Z0-9_]`)
	pattern = reNonDigitChar.ReplaceAllString(pattern, `[^0-9]`)
	pattern = reDigitChar.ReplaceAllString(pattern, `[0-9]`)
	pattern = reNonSpaceChar.ReplaceAllString(pattern, `[^ \t\n\r\f\v]`)
	pattern = reSpaceChar.ReplaceAllString(pattern, `[ \t\n\r\f\v]`)

	// Approximate {n,} with +
	pattern = reNumericQuantifierOpen.ReplaceAllString(pattern, `+`)

	return pattern
}
