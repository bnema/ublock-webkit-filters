package converter

import (
	"strings"
	"testing"

	"github.com/bnema/ublock-webkit-filters/internal/models"
	"github.com/bnema/ublock-webkit-filters/internal/parser"
	"github.com/stretchr/testify/assert"
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
