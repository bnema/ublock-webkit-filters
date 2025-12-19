package models

import "time"

// Config represents the main configuration
type Config struct {
	HTTP   HTTPConfig   `mapstructure:"http"`
	Output OutputConfig `mapstructure:"output"`
	Lists  []FilterList `mapstructure:"lists"`
}

// HTTPConfig contains HTTP client settings
type HTTPConfig struct {
	Timeout time.Duration `mapstructure:"timeout"`
	Retries int           `mapstructure:"retries"`
}

// OutputConfig contains output settings
type OutputConfig struct {
	MaxRulesPerFile  int  `mapstructure:"max_rules_per_file"`
	GenerateCombined bool `mapstructure:"generate_combined"`
	GenerateManifest bool `mapstructure:"generate_manifest"`
}

// FilterList represents a single filter list configuration
type FilterList struct {
	Name    string `mapstructure:"name"`
	URL     string `mapstructure:"url"`
	Enabled bool   `mapstructure:"enabled"`
}

// EnabledLists returns only enabled filter lists
func (c *Config) EnabledLists() []FilterList {
	var enabled []FilterList
	for _, l := range c.Lists {
		if l.Enabled {
			enabled = append(enabled, l)
		}
	}
	return enabled
}
