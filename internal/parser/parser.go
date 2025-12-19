package parser

import (
	"bufio"
	"io"
	"strings"

	"github.com/bnema/ublock-webkit-filters/internal/models"
)

// Parser parses ABP/uBlock filter lists
type Parser struct {
	stats Stats
}

// Stats tracks parsing statistics
type Stats struct {
	Total       int
	Network     int
	Exception   int
	Cosmetic    int
	Comments    int
	Unsupported int
	SkipReasons map[string]int // Detailed breakdown of skipped filters
}

// SkipReason constants
const (
	SkipScriptlet        = "scriptlet (##+js)"
	SkipHTMLFilter       = "html-filter (##^)"
	SkipProcedural       = "procedural (:has, :xpath, etc)"
	SkipUnsupportedOpt   = "unsupported-option (redirect, csp, etc)"
	SkipInvalidRegex     = "invalid-regex"
	SkipCosmeticException = "cosmetic-exception (#@#)"
)

// New creates a new parser
func New() *Parser {
	return &Parser{
		stats: Stats{
			SkipReasons: make(map[string]int),
		},
	}
}

// skip records a skipped filter with reason
func (p *Parser) skip(reason string) models.Filter {
	p.stats.SkipReasons[reason]++
	return models.Filter{Type: models.FilterTypeUnsupported}
}

// Stats returns parsing statistics
func (p *Parser) Stats() Stats {
	return p.stats
}

// Parse reads filter content and returns parsed filters
func (p *Parser) Parse(r io.Reader) ([]models.Filter, error) {
	var filters []models.Filter
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		filter := p.parseLine(line)
		p.stats.Total++

		switch filter.Type {
		case models.FilterTypeComment:
			p.stats.Comments++
			continue // skip comments
		case models.FilterTypeUnsupported:
			p.stats.Unsupported++
			continue // skip unsupported
		case models.FilterTypeNetwork:
			p.stats.Network++
		case models.FilterTypeException:
			p.stats.Exception++
		case models.FilterTypeCosmetic, models.FilterTypeCosmeticException:
			p.stats.Cosmetic++
		}

		filters = append(filters, filter)
	}

	return filters, scanner.Err()
}

// parseLine parses a single filter line
func (p *Parser) parseLine(line string) models.Filter {
	// Comments
	if strings.HasPrefix(line, "!") || strings.HasPrefix(line, "[") {
		return models.Filter{Type: models.FilterTypeComment, Raw: line}
	}

	// Scriptlet injection - unsupported
	if strings.Contains(line, "##+js(") || strings.Contains(line, "#@#+js(") {
		return p.skip(SkipScriptlet)
	}

	// HTML filtering - unsupported
	if strings.Contains(line, "##^") || strings.Contains(line, "#@#^") {
		return p.skip(SkipHTMLFilter)
	}

	// Procedural cosmetic filters - unsupported
	if containsProcedural(line) {
		return p.skip(SkipProcedural)
	}

	// Cosmetic filters
	if idx := strings.Index(line, "##"); idx != -1 && !strings.Contains(line, "#@#") {
		return p.parseCosmetic(line, idx, false)
	}

	// Cosmetic exception filters
	if idx := strings.Index(line, "#@#"); idx != -1 {
		return p.parseCosmetic(line, idx, true)
	}

	// Exception rules (whitelist)
	if strings.HasPrefix(line, "@@") {
		return p.parseNetwork(line[2:], true)
	}

	// Network filters
	return p.parseNetwork(line, false)
}

// containsProcedural checks for procedural cosmetic filter syntax
func containsProcedural(line string) bool {
	procedural := []string{
		":has(", ":has-text(", ":xpath(", ":matches-css(",
		":matches-attr(", ":min-text-length(", ":not(",
		":upward(", ":remove(", ":style(",
	}
	for _, p := range procedural {
		if strings.Contains(line, p) {
			return true
		}
	}
	return false
}

// parseCosmetic parses a cosmetic (CSS) filter
func (p *Parser) parseCosmetic(line string, sepIdx int, isException bool) models.Filter {
	separator := "##"
	filterType := models.FilterTypeCosmetic
	if isException {
		separator = "#@#"
		filterType = models.FilterTypeCosmeticException
	}

	var domains []string
	if sepIdx > 0 {
		domainPart := line[:sepIdx]
		domains = parseDomainList(domainPart)
	}

	selector := line[sepIdx+len(separator):]

	return models.Filter{
		Type:     filterType,
		Raw:      line,
		Selector: selector,
		Domains:  domains,
	}
}

// parseNetwork parses a network filter
func (p *Parser) parseNetwork(line string, isException bool) models.Filter {
	filterType := models.FilterTypeNetwork
	if isException {
		filterType = models.FilterTypeException
	}

	pattern := line
	var options models.FilterOptions

	// Split pattern and options
	if idx := strings.LastIndex(line, "$"); idx != -1 {
		// Check it's not escaped or part of regex
		if idx == 0 || line[idx-1] != '\\' {
			optPart := line[idx+1:]
			// Skip if it looks like a regex end anchor
			if !strings.HasPrefix(optPart, "/") {
				pattern = line[:idx]
				options = parseOptions(optPart)

				// Check for unsupported options
				if hasUnsupportedOptions(optPart) {
					return p.skip(SkipUnsupportedOpt)
				}
			}
		}
	}

	return models.Filter{
		Type:    filterType,
		Raw:     line,
		Pattern: pattern,
		Options: options,
	}
}

// parseDomainList parses comma-separated domain list
func parseDomainList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	domains := make([]string, 0, len(parts))
	for _, d := range parts {
		d = strings.TrimSpace(d)
		if d != "" {
			domains = append(domains, d)
		}
	}
	return domains
}

// parseOptions parses network filter options
func parseOptions(s string) models.FilterOptions {
	var opts models.FilterOptions
	parts := strings.Split(s, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		switch {
		case part == "third-party" || part == "3p":
			t := true
			opts.ThirdParty = &t
		case part == "~third-party" || part == "~3p" || part == "first-party" || part == "1p":
			f := false
			opts.ThirdParty = &f
		case part == "match-case":
			opts.MatchCase = true
		case part == "important":
			opts.Important = true
		case strings.HasPrefix(part, "domain="):
			opts.Domains, opts.ExcludeDomains = parseDomainOption(part[7:])
		default:
			// Check if it's a resource type
			if rt := mapResourceType(part); rt != "" {
				opts.ResourceTypes = append(opts.ResourceTypes, rt)
			}
		}
	}

	return opts
}

// parseDomainOption parses domain=example.com|~excluded.com
func parseDomainOption(s string) (include, exclude []string) {
	parts := strings.Split(s, "|")
	for _, d := range parts {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		if strings.HasPrefix(d, "~") {
			exclude = append(exclude, d[1:])
		} else {
			include = append(include, d)
		}
	}
	return
}

// mapResourceType maps ABP resource types to WebKit types
func mapResourceType(s string) string {
	// Handle negation
	s = strings.TrimPrefix(s, "~")

	switch s {
	case "script":
		return models.ResourceScript
	case "image", "img":
		return models.ResourceImage
	case "stylesheet", "css":
		return models.ResourceStyleSheet
	case "font":
		return models.ResourceFont
	case "media":
		return models.ResourceMedia
	case "xmlhttprequest", "xhr":
		return models.ResourceRaw
	case "subdocument", "frame":
		return models.ResourceDocument
	case "object", "object-subrequest":
		return models.ResourceRaw
	case "ping", "beacon":
		return models.ResourceRaw
	case "popup":
		return models.ResourcePopup
	case "other":
		return models.ResourceRaw
	case "websocket":
		return models.ResourceRaw
	case "document", "doc":
		return models.ResourceDocument
	}
	return ""
}

// hasUnsupportedOptions checks for options that can't be converted
func hasUnsupportedOptions(s string) bool {
	unsupported := []string{
		"redirect=", "redirect-rule=",
		"csp=", "removeparam=", "replace=",
		"header=", "method=", "to=",
		"permissions=", "uritransform=",
	}
	for _, u := range unsupported {
		if strings.Contains(s, u) {
			return true
		}
	}
	return false
}
