package converter

import (
	"testing"

	"github.com/bnema/ublock-webkit-filters/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestNetworkFilterWithIncludeAndExcludeDomainsIsSkipped(t *testing.T) {
	tp := true
	f := models.Filter{
		Type:    models.FilterTypeNetwork,
		Pattern: "*",
		Options: models.FilterOptions{
			ThirdParty:     &tp,
			ResourceTypes:  []string{models.ResourceScript},
			Domains:        []string{"pingit.com"},
			ExcludeDomains: []string{"pingit.tel"},
		},
	}

	c := New()
	rules := c.Convert([]models.Filter{f})

	assert.Empty(t, rules, "Filter with both include and exclude domains must be skipped")
	assert.Equal(t, 1, c.stats.Skipped)
}

func TestNetworkFilterWithOnlyIncludeDomainsIsConverted(t *testing.T) {
	tp := true
	f := models.Filter{
		Type:    models.FilterTypeNetwork,
		Pattern: "||ads.example.com^",
		Options: models.FilterOptions{
			ThirdParty: &tp,
			Domains:    []string{"example.com"},
		},
	}

	c := New()
	rules := c.Convert([]models.Filter{f})

	assert.NotEmpty(t, rules, "Filter with only include domains should be converted")
	assert.Equal(t, []string{"*example.com"}, rules[0].Trigger.IfDomain)
}

func TestNetworkFilterWithOnlyExcludeDomainsIsConverted(t *testing.T) {
	tp := true
	f := models.Filter{
		Type:    models.FilterTypeNetwork,
		Pattern: "||ads.example.com^",
		Options: models.FilterOptions{
			ThirdParty:     &tp,
			ExcludeDomains: []string{"safe.com"},
		},
	}

	c := New()
	rules := c.Convert([]models.Filter{f})

	assert.NotEmpty(t, rules, "Filter with only exclude domains should be converted")
	assert.Equal(t, []string{"*safe.com"}, rules[0].Trigger.UnlessDomain)
}

func TestEntityMatchingDomainIsSkipped(t *testing.T) {
	tests := []struct {
		name    string
		domains []string
	}{
		{"single entity", []string{"pingit.*"}},
		{"multiple entities", []string{"mylink.*", "my1ink.*", "myl1nk.*"}},
		{"mixed entity and normal", []string{"example.com", "pingit.*"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tp := true
			f := models.Filter{
				Type:    models.FilterTypeNetwork,
				Pattern: "*",
				Options: models.FilterOptions{
					ThirdParty:    &tp,
					ResourceTypes: []string{models.ResourceScript},
					Domains:       tt.domains,
				},
			}

			c := New()
			rules := c.Convert([]models.Filter{f})

			assert.Empty(t, rules, "Filter with entity matching domain should be skipped")
			assert.Equal(t, 1, c.stats.Skipped)
		})
	}
}

func TestNormalDomainIsNotSkipped(t *testing.T) {
	tp := true
	f := models.Filter{
		Type:    models.FilterTypeNetwork,
		Pattern: "||ads.com^",
		Options: models.FilterOptions{
			ThirdParty: &tp,
			Domains:    []string{"example.com", "test.org"},
		},
	}

	c := New()
	rules := c.Convert([]models.Filter{f})

	assert.NotEmpty(t, rules, "Filter with normal domains should be converted")
}

func TestCosmeticFilterWithIncludeAndExcludeDomainsIsSkipped(t *testing.T) {
	f := models.Filter{
		Type:     models.FilterTypeCosmetic,
		Selector: ".ad-banner",
		Domains:  []string{"example.com", "~safe.example.com"},
	}

	c := New()
	rules := c.Convert([]models.Filter{f})

	assert.Empty(t, rules, "Cosmetic filter with both include and exclude domains must be skipped")
	assert.Equal(t, 1, c.stats.Skipped)
}
