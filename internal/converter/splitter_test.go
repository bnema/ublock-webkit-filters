package converter

import (
	"testing"

	"github.com/bnema/ublock-webkit-filters/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestDeduplicateKeepsDistinctRulesWithSameURLFilter(t *testing.T) {
	rules := []models.WebKitRule{
		{
			Trigger: models.WebKitTrigger{
				URLFilter:    ".*",
				ResourceType: []string{"image"},
				LoadType:     []string{"third-party"},
			},
			Action: models.WebKitAction{Type: "block"},
		},
		{
			Trigger: models.WebKitTrigger{
				URLFilter:    ".*",
				ResourceType: []string{"script"},
				LoadType:     []string{"third-party"},
			},
			Action: models.WebKitAction{Type: "block"},
		},
	}

	result := Deduplicate(rules)

	assert.Len(t, result, 2, "Rules with different ResourceType must not be deduplicated")
}

func TestDeduplicateKeepsDistinctDomains(t *testing.T) {
	rules := []models.WebKitRule{
		{
			Trigger: models.WebKitTrigger{
				URLFilter: ".*",
				IfDomain:  []string{"*example.com"},
			},
			Action: models.WebKitAction{Type: "block"},
		},
		{
			Trigger: models.WebKitTrigger{
				URLFilter: ".*",
				IfDomain:  []string{"*other.com"},
			},
			Action: models.WebKitAction{Type: "block"},
		},
	}

	result := Deduplicate(rules)

	assert.Len(t, result, 2, "Rules with different IfDomain must not be deduplicated")
}

func TestDeduplicateRemovesTrueDuplicates(t *testing.T) {
	rules := []models.WebKitRule{
		{
			Trigger: models.WebKitTrigger{
				URLFilter:    ".*",
				ResourceType: []string{"script"},
				LoadType:     []string{"third-party"},
				IfDomain:     []string{"*example.com"},
			},
			Action: models.WebKitAction{Type: "block"},
		},
		{
			Trigger: models.WebKitTrigger{
				URLFilter:    ".*",
				ResourceType: []string{"script"},
				LoadType:     []string{"third-party"},
				IfDomain:     []string{"*example.com"},
			},
			Action: models.WebKitAction{Type: "block"},
		},
	}

	result := Deduplicate(rules)

	assert.Len(t, result, 1, "Identical rules should be deduplicated")
}

func TestDeduplicateDistinguishesCaseSensitivity(t *testing.T) {
	cs := true
	rules := []models.WebKitRule{
		{
			Trigger: models.WebKitTrigger{URLFilter: "test"},
			Action:  models.WebKitAction{Type: "block"},
		},
		{
			Trigger: models.WebKitTrigger{URLFilter: "test", URLFilterIsCaseSensitive: &cs},
			Action:  models.WebKitAction{Type: "block"},
		},
	}

	result := Deduplicate(rules)

	assert.Len(t, result, 2, "Rules with different case-sensitivity must not be deduplicated")
}
