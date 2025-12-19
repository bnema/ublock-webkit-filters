# ublock-webkit-filters

Convert [uBlock Origin](https://github.com/gorhill/uBlock) filter lists to Safari/WebKitGTK content blocker JSON format.

## Overview

This tool fetches popular ad-blocking filter lists and converts them to the JSON format used by:
- Safari Content Blockers
- WebKitGTK `UserContentFilterStore`

Filters are updated daily via GitHub Actions and published as release artifacts.

## Download

Get the latest converted filters from [Releases](https://github.com/bnema/ublock-webkit-filters/releases/latest).

### Files

| File | Description |
|------|-------------|
| `combined-part1.json`, `combined-part2.json`, ... | All lists merged and deduplicated (split at 50k rules) |
| `easylist.json` | EasyList - ad blocking |
| `easyprivacy.json` | EasyPrivacy - tracker blocking |
| `ublock-filters.json` | uBlock Origin optimizations |
| `manifest.json` | Metadata with rule counts |
| `checksums.txt` | SHA256 checksums |

## Usage with WebKitGTK

```go
import "github.com/user/puregotk-webkit/webkit"

// Create filter store
store := webkit.NewUserContentFilterStore("/path/to/filters")

// Load JSON filter
store.Save("combined", jsonData, nil, func(filter *webkit.UserContentFilter, err error) {
    if err != nil {
        log.Fatal(err)
    }
    // Add to content manager
    contentManager.AddFilter(filter)
})
```

## Building

```bash
go build -o ublock-webkit-filters ./cmd/ublock-webkit-filters
```

## Commands

### Convert filters

```bash
# Convert all enabled lists
./ublock-webkit-filters convert --output ./output

# Dry run (parse and convert without writing files)
./ublock-webkit-filters convert --dry-run

# Verbose output
./ublock-webkit-filters convert --output ./output --verbose
```

### List configured filters

```bash
./ublock-webkit-filters list
```

### Create default config

```bash
./ublock-webkit-filters init
```

## Configuration

Edit `configs/filter_lists.toml`:

```toml
[http]
timeout = "30s"
retries = 3

[output]
max_rules_per_file = 50000
generate_combined = true
generate_manifest = true

[[lists]]
name = "easylist"
url = "https://easylist.to/easylist/easylist.txt"
enabled = true

[[lists]]
name = "easyprivacy"
url = "https://easylist.to/easylist/easyprivacy.txt"
enabled = true

# Add more lists...
```

## Filter Conversion

### Supported

| uBlock Syntax | WebKit Action |
|---------------|---------------|
| `\|\|ads.com^` | `block` |
| `@@\|\|safe.com` | `ignore-previous-rules` |
| `##.ad-banner` | `css-display-none` |
| `$third-party` | `load-type: third-party` |
| `$script,image` | `resource-type` |

### Not Supported (skipped)

- Scriptlet injection: `##+js(...)`
- HTML filtering: `##^`
- Procedural cosmetic: `:has()`, `:has-text()`, `:xpath()`
- Redirects, CSP, removeparam

## Default Filter Lists

- [EasyList](https://easylist.to/) - Ad blocking
- [EasyPrivacy](https://easylist.to/) - Tracker blocking
- [uBlock Origin filters](https://github.com/uBlockOrigin/uAssets) - Optimizations
- [Peter Lowe's Ad server list](https://pgl.yoyo.org/adservers/)

## License

MIT
