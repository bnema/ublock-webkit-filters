package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExpandCharacterClasses(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "expand \\w",
			input:    `\w`,
			expected: `[a-zA-Z0-9_]`,
		},
		{
			name:     "expand \\W",
			input:    `\W`,
			expected: `[^a-zA-Z0-9_]`,
		},
		{
			name:     "expand \\d",
			input:    `\d`,
			expected: `[0-9]`,
		},
		{
			name:     "expand \\D",
			input:    `\D`,
			expected: `[^0-9]`,
		},
		{
			name:     "expand \\s",
			input:    `\s`,
			expected: `[ \t\n\r\f\v]`,
		},
		{
			name:     "expand \\S",
			input:    `\S`,
			expected: `[^ \t\n\r\f\v]`,
		},
		{
			name:     "expand multiple \\w with quantifier",
			input:    `(https?:\/\/)\w{30,}\.me\/\w{30,}\.`,
			expected: `(https?:\/\/)[a-zA-Z0-9_]{30,}\.me\/[a-zA-Z0-9_]{30,}\.`,
		},
		{
			name:     "expand mixed character classes",
			input:    `\w\d\s`,
			expected: `[a-zA-Z0-9_][0-9][ \t\n\r\f\v]`,
		},
		{
			name:     "no expansion needed",
			input:    `[a-z]+\.example\.com`,
			expected: `[a-z]+\.example\.com`,
		},
		{
			name:     "preserve literal backslash-w in character class",
			input:    `[\w]`,
			expected: `[[a-zA-Z0-9_]]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandCharacterClasses(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPatternToRegex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty pattern",
			input:    "",
			expected: ".*",
		},
		{
			name:     "wildcard only",
			input:    "*",
			expected: ".*",
		},
		{
			name:     "simple pattern",
			input:    "example.com",
			expected: `example\.com`,
		},
		{
			name:     "hostname anchor",
			input:    "||ads.example.com",
			expected: `^[a-z-]+://(?:[^/?#]+\.)?ads\.example\.com`,
		},
		{
			name:     "hostname anchor with dot prefix",
			input:    "||.example.com",
			expected: `^[a-z-]+://(?:[^/?#]+)?\.example\.com`,
		},
		{
			name:     "separator",
			input:    "||example.com^",
			expected: `^[a-z-]+://(?:[^/?#]+\.)?example\.com[^%.0-9a-z_-]`,
		},
		{
			name:     "left anchor",
			input:    "|http://example.com",
			expected: `^http://example\.com`,
		},
		{
			name:     "right anchor",
			input:    "example.com/path|",
			expected: `example\.com/path$`,
		},
		{
			name:     "both anchors",
			input:    "|http://example.com/|",
			expected: `^http://example\.com/$`,
		},
		{
			name:     "regex pattern with \\w (expands but has numeric quantifier)",
			input:    `/(https?:\/\/)\w{30,}\.me\/\w{30,}\./`,
			expected: `(https?:\/\/)[a-zA-Z0-9_]{30,}\.me\/[a-zA-Z0-9_]{30,}\.`,
		},
		{
			name:     "regex pattern with \\w and basic quantifier",
			input:    `/(https?:\/\/)\w+\.me\/\w+\./`,
			expected: `(https?:\/\/)[a-zA-Z0-9_]+\.me\/[a-zA-Z0-9_]+\.`,
		},
		{
			name:     "regex pattern with \\d",
			input:    `/api\/v\d+/`,
			expected: `api\/v[0-9]+`,
		},
		{
			name:     "regex pattern with \\s",
			input:    `/\s+/`,
			expected: `[ \t\n\r\f\v]+`,
		},
		{
			name:     "regex pattern without expansion needed",
			input:    `/[a-z]+\.example\.com/`,
			expected: `[a-z]+\.example\.com`,
		},
		{
			name:     "wildcard in middle",
			input:    "||example.com^*path",
			expected: `^[a-z-]+://(?:[^/?#]+\.)?example\.com[^%.0-9a-z_-].*path`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PatternToRegex(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateRegex(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid simple regex",
			input:    `example\.com`,
			expected: true,
		},
		{
			name:     "valid character class",
			input:    `[a-zA-Z0-9_]+`,
			expected: true,
		},
		{
			name:     "invalid - numeric quantifier",
			input:    `[a-zA-Z0-9_]{30,}`,
			expected: false,
		},
		{
			name:     "invalid - exact numeric quantifier",
			input:    `[0-9]{4}`,
			expected: false,
		},
		{
			name:     "invalid - disjunction",
			input:    `foo|bar`,
			expected: false,
		},
		{
			name:     "valid - pipe in character class",
			input:    `[a|b]`,
			expected: true,
		},
		{
			name:     "invalid - negative lookbehind",
			input:    `(?<!foo)bar`,
			expected: false,
		},
		{
			name:     "invalid - positive lookbehind",
			input:    `(?<=foo)bar`,
			expected: false,
		},
		{
			name:     "invalid - positive lookahead",
			input:    `foo(?=bar)`,
			expected: false,
		},
		{
			name:     "invalid - negative lookahead",
			input:    `foo(?!bar)`,
			expected: false,
		},
		{
			name:     "invalid - unicode property",
			input:    `\p{L}`,
			expected: false,
		},
		{
			name:     "invalid - word boundary",
			input:    `\bword\b`,
			expected: false,
		},
		{
			name:     "valid - basic quantifiers",
			input:    `[a-z]+`,
			expected: true,
		},
		{
			name:     "valid - star quantifier",
			input:    `.*`,
			expected: true,
		},
		{
			name:     "valid - question quantifier",
			input:    `https?`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateRegex(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPatternEndsWithSeparator(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "ends with separator",
			input:    "||example.com^",
			expected: true,
		},
		{
			name:     "ends with separator and right anchor",
			input:    "||example.com^|",
			expected: true,
		},
		{
			name:     "no separator",
			input:    "||example.com",
			expected: false,
		},
		{
			name:     "separator in middle",
			input:    "||example.com^path",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PatternEndsWithSeparator(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPatternToRegexEndAnchor(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "separator becomes end anchor",
			input:    "||example.com^",
			expected: `^[a-z-]+://(?:[^/?#]+\.)?example\.com$`,
		},
		{
			name:     "with right anchor too",
			input:    "||example.com^|",
			expected: `^[a-z-]+://(?:[^/?#]+\.)?example\.com$`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PatternToRegexEndAnchor(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContainsDisjunction(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "no pipe",
			input:    `example\.com`,
			expected: false,
		},
		{
			name:     "pipe in character class",
			input:    `[a|b]`,
			expected: false,
		},
		{
			name:     "pipe outside character class",
			input:    `foo|bar`,
			expected: true,
		},
		{
			name:     "escaped pipe",
			input:    `foo\|bar`,
			expected: false,
		},
		{
			name:     "complex pattern with pipe in class",
			input:    `^[a-z-]+://(?:[^/?#|]+)?`,
			expected: false,
		},
		{
			name:     "disjunction after character class",
			input:    `[abc]|def`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsDisjunction(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
