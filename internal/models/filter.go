package models

// FilterType represents the type of filter parsed
type FilterType int

const (
	FilterTypeComment FilterType = iota
	FilterTypeNetwork
	FilterTypeException
	FilterTypeCosmetic
	FilterTypeCosmeticException
	FilterTypeUnsupported // scriptlets, HTML filters, procedural
)

// Filter represents a parsed ABP/uBlock filter
type Filter struct {
	Type     FilterType
	Raw      string        // Original filter line
	Pattern  string        // URL pattern for network filters
	Selector string        // CSS selector for cosmetic filters
	Domains  []string      // Domains this filter applies to
	Options  FilterOptions // Network filter options
}

// FilterOptions contains parsed network filter options
type FilterOptions struct {
	ThirdParty     *bool    // nil = any, true = 3p only, false = 1p only
	ResourceTypes  []string // script, image, stylesheet, etc.
	Domains        []string // domain= values (apply to these domains)
	ExcludeDomains []string // ~domain values (exclude these domains)
	MatchCase      bool     // case-sensitive matching
	Important      bool     // override exceptions
}

// IsEmpty returns true if no options are set
func (o FilterOptions) IsEmpty() bool {
	return o.ThirdParty == nil &&
		len(o.ResourceTypes) == 0 &&
		len(o.Domains) == 0 &&
		len(o.ExcludeDomains) == 0 &&
		!o.MatchCase &&
		!o.Important
}
