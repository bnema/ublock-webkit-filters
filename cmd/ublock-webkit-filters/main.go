package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bnema/ublock-webkit-filters/internal/converter"
	"github.com/bnema/ublock-webkit-filters/internal/fetcher"
	"github.com/bnema/ublock-webkit-filters/internal/models"
	"github.com/bnema/ublock-webkit-filters/internal/parser"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	cfg     models.Config
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "ublock-webkit-filters",
	Short: "Convert uBlock filter lists to WebKit content blocker format",
	Long: `A tool that converts uBlock Origin filter lists to Safari/WebKitGTK
compatible content blocker JSON format.`,
}

var convertCmd = &cobra.Command{
	Use:   "convert",
	Short: "Convert filter lists to WebKit JSON format",
	RunE:  runConvert,
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured filter lists",
	RunE:  runList,
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default config file",
	RunE:  runInit,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: ./configs/filter_lists.toml)")

	convertCmd.Flags().StringP("output", "o", "./output", "output directory")
	convertCmd.Flags().Bool("dry-run", false, "parse and convert without writing files")
	convertCmd.Flags().Bool("combined", true, "generate combined output file")
	convertCmd.Flags().Bool("verbose", false, "verbose output")

	rootCmd.AddCommand(convertCmd, listCmd, initCmd)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("filter_lists")
		viper.SetConfigType("toml")
		viper.AddConfigPath("./configs")
		viper.AddConfigPath(".")
	}

	// Set defaults
	viper.SetDefault("http.timeout", "30s")
	viper.SetDefault("http.retries", 3)
	viper.SetDefault("output.max_rules_per_file", 50000)
	viper.SetDefault("output.generate_combined", true)
	viper.SetDefault("output.generate_manifest", true)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
		}
	}

	if err := viper.Unmarshal(&cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing config: %v\n", err)
	}
}

func runConvert(cmd *cobra.Command, args []string) error {
	outputDir, _ := cmd.Flags().GetString("output")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	generateCombined, _ := cmd.Flags().GetBool("combined")
	verbose, _ := cmd.Flags().GetBool("verbose")

	enabledLists := cfg.EnabledLists()
	if len(enabledLists) == 0 {
		return fmt.Errorf("no enabled filter lists found in config")
	}

	fmt.Printf("Converting %d filter lists...\n", len(enabledLists))
	if dryRun {
		fmt.Println("[DRY RUN] No files will be written")
	}

	ctx := context.Background()
	f := fetcher.New(cfg.HTTP)
	splitter := converter.NewSplitter(cfg.Output.MaxRulesPerFile)

	var allRules []models.WebKitRule
	results := make(map[string]ListResult)

	// Aggregate skip reasons across all lists
	totalParseSkips := make(map[string]int)
	totalConvertSkips := make(map[string]int)

	for _, list := range enabledLists {
		fmt.Printf("\n  Processing %s...\n", list.Name)

		// Fetch
		data, err := f.Fetch(ctx, list.URL)
		if err != nil {
			fmt.Printf("    ERROR: %v\n", err)
			continue
		}
		fmt.Printf("    Downloaded: %d bytes\n", len(data))

		// Parse (fresh parser per list for accurate stats)
		p := parser.New()
		filters, err := p.Parse(bytes.NewReader(data))
		if err != nil {
			fmt.Printf("    ERROR parsing: %v\n", err)
			continue
		}
		pStats := p.Stats()

		// Convert (fresh converter per list for accurate stats)
		c := converter.New()
		rules := c.Convert(filters)
		cStats := c.Stats()

		totalSkipped := pStats.Unsupported + cStats.Skipped
		fmt.Printf("    Converted: %d rules (skipped: %d)\n", len(rules), totalSkipped)

		if verbose {
			fmt.Printf("    Parsed: %d total, %d network, %d cosmetic, %d exceptions\n",
				pStats.Total, pStats.Network, pStats.Cosmetic, pStats.Exception)
			if len(pStats.SkipReasons) > 0 {
				fmt.Printf("    Parse skips:\n")
				for reason, count := range pStats.SkipReasons {
					fmt.Printf("      - %s: %d\n", reason, count)
					totalParseSkips[reason] += count
				}
			}
			if len(cStats.SkipReasons) > 0 {
				fmt.Printf("    Convert skips:\n")
				for reason, count := range cStats.SkipReasons {
					fmt.Printf("      - %s: %d\n", reason, count)
					totalConvertSkips[reason] += count
				}
			}
		} else {
			// Still aggregate for summary
			for reason, count := range pStats.SkipReasons {
				totalParseSkips[reason] += count
			}
			for reason, count := range cStats.SkipReasons {
				totalConvertSkips[reason] += count
			}
		}

		results[list.Name] = ListResult{
			Name:         list.Name,
			URL:          list.URL,
			RulesCount:   len(rules),
			SkippedCount: totalSkipped,
		}

		if !dryRun {
			// Split and write
			parts := splitter.Split(rules, list.Name)
			for name, partRules := range parts {
				if err := writeJSON(outputDir, name+".json", partRules); err != nil {
					fmt.Printf("    ERROR writing %s: %v\n", name, err)
				}
			}
		}

		allRules = append(allRules, rules...)
	}

	// Show skip summary
	if len(totalParseSkips) > 0 || len(totalConvertSkips) > 0 {
		fmt.Printf("\nSkipped filters summary:\n")
		for reason, count := range totalParseSkips {
			fmt.Printf("  %s: %d\n", reason, count)
		}
		for reason, count := range totalConvertSkips {
			fmt.Printf("  %s: %d\n", reason, count)
		}
	}

	// Deduplicate combined rules
	if generateCombined && len(allRules) > 0 {
		fmt.Printf("\nGenerating combined output...\n")
		allRules = converter.Deduplicate(allRules)
		fmt.Printf("  Total rules: %d (after deduplication)\n", len(allRules))

		if !dryRun {
			parts := splitter.Split(allRules, "combined")
			var partNames []string
			for name, partRules := range parts {
				if err := writeJSON(outputDir, name+".json", partRules); err != nil {
					fmt.Printf("  ERROR writing %s: %v\n", name, err)
				}
				partNames = append(partNames, name+".json")
			}

			// Write manifest
			if cfg.Output.GenerateManifest {
				manifest := Manifest{
					Version:     time.Now().Format("2006.01.02"),
					GeneratedAt: time.Now().UTC().Format(time.RFC3339),
					Lists:       results,
					Combined: CombinedInfo{
						TotalRules: len(allRules),
						Files:      partNames,
					},
				}
				if err := writeJSON(outputDir, "manifest.json", manifest); err != nil {
					fmt.Printf("  ERROR writing manifest: %v\n", err)
				}
			}
		}
	}

	fmt.Println("\nDone!")
	return nil
}

func runList(cmd *cobra.Command, args []string) error {
	fmt.Println("Configured filter lists:\n")
	for _, list := range cfg.Lists {
		status := "enabled"
		if !list.Enabled {
			status = "disabled"
		}
		fmt.Printf("  [%s] %s\n", status, list.Name)
		fmt.Printf("         %s\n\n", list.URL)
	}
	return nil
}

func runInit(cmd *cobra.Command, args []string) error {
	configPath := "./configs/filter_lists.toml"
	if cfgFile != "" {
		configPath = cfgFile
	}

	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config file already exists: %s", configPath)
	}

	// Copy embedded default config
	defaultConfig := `# uBlock to WebKit Filters Converter Configuration

# HTTP client settings
[http]
timeout = "30s"
retries = 3

# Output settings
[output]
max_rules_per_file = 50000
generate_combined = true
generate_manifest = true

# Filter lists to convert
# Set enabled = false to skip a list

[[lists]]
name = "easylist"
url = "https://easylist.to/easylist/easylist.txt"
enabled = true

[[lists]]
name = "easyprivacy"
url = "https://easylist.to/easylist/easyprivacy.txt"
enabled = true

[[lists]]
name = "ublock-filters"
url = "https://ublockorigin.github.io/uAssets/filters/filters.txt"
enabled = true

[[lists]]
name = "ublock-privacy"
url = "https://ublockorigin.github.io/uAssets/filters/privacy.txt"
enabled = true

[[lists]]
name = "ublock-badware"
url = "https://ublockorigin.github.io/uAssets/filters/badware.txt"
enabled = true

[[lists]]
name = "ublock-unbreak"
url = "https://ublockorigin.github.io/uAssets/filters/unbreak.txt"
enabled = true

[[lists]]
name = "ublock-quick-fixes"
url = "https://ublockorigin.github.io/uAssets/filters/quick-fixes.txt"
enabled = true

[[lists]]
name = "peter-lowe"
url = "https://pgl.yoyo.org/adservers/serverlist.php?hostformat=hosts&showintro=1&mimetype=plaintext"
enabled = true
`

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return err
	}

	fmt.Printf("Created config file: %s\n", configPath)
	return nil
}

func writeJSON(dir, filename string, data any) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, filename)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// ListResult contains conversion results for a single list
type ListResult struct {
	Name         string `json:"name"`
	URL          string `json:"source_url"`
	RulesCount   int    `json:"rules_count"`
	SkippedCount int    `json:"skipped_count"`
}

// Manifest contains metadata about the conversion
type Manifest struct {
	Version     string                `json:"version"`
	GeneratedAt string                `json:"generated_at"`
	Lists       map[string]ListResult `json:"lists"`
	Combined    CombinedInfo          `json:"combined"`
}

// CombinedInfo contains combined file info
type CombinedInfo struct {
	TotalRules int      `json:"total_rules"`
	Files      []string `json:"files"`
}
