package models

// WebKitRule represents a Safari/WebKit content blocker rule
type WebKitRule struct {
	Trigger WebKitTrigger `json:"trigger"`
	Action  WebKitAction  `json:"action"`
}

// WebKitTrigger defines when a rule should activate
type WebKitTrigger struct {
	URLFilter                string   `json:"url-filter"`
	URLFilterIsCaseSensitive *bool    `json:"url-filter-is-case-sensitive,omitempty"`
	ResourceType             []string `json:"resource-type,omitempty"`
	LoadType                 []string `json:"load-type,omitempty"`
	IfDomain                 []string `json:"if-domain,omitempty"`
	UnlessDomain             []string `json:"unless-domain,omitempty"`
}

// WebKitAction defines what to do when a rule triggers
type WebKitAction struct {
	Type     string `json:"type"`               // block, block-cookies, css-display-none, ignore-previous-rules
	Selector string `json:"selector,omitempty"` // only for css-display-none
}

// Action type constants
const (
	ActionBlock              = "block"
	ActionBlockCookies       = "block-cookies"
	ActionCSSDisplayNone     = "css-display-none"
	ActionIgnorePreviousRule = "ignore-previous-rules"
)

// Resource type constants (WebKit names)
const (
	ResourceDocument   = "document"
	ResourceImage      = "image"
	ResourceStyleSheet = "style-sheet"
	ResourceScript     = "script"
	ResourceFont       = "font"
	ResourceRaw        = "raw"
	ResourceSVG        = "svg-document"
	ResourceMedia      = "media"
	ResourcePopup      = "popup"
)

// Load type constants
const (
	LoadFirstParty = "first-party"
	LoadThirdParty = "third-party"
)
