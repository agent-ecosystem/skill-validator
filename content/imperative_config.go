package content

import (
	"embed"

	"gopkg.in/yaml.v3"
)

//go:embed imperative_default.yaml
var defaultConfigFS embed.FS

// ImperativeConfig holds the multilingual imperative detection configuration.
// Library consumers can construct one programmatically and pass it to
// AnalyzeWithConfig to extend or replace the embedded defaults.
type ImperativeConfig struct {
	// Languages maps a language tag (e.g. "en", "zh") to its detection rules.
	Languages map[string]LanguageRules `yaml:"languages"`
}

// LanguageRules defines imperative detection rules for one language.
type LanguageRules struct {
	// Verbs is a list of imperative verbs/phrases to match at sentence start.
	Verbs []string `yaml:"verbs"`

	// Keywords is a list of imperative keywords/phrases matched anywhere in
	// the sentence (useful for languages like Chinese where imperatives don't
	// always start with a verb).
	Keywords []string `yaml:"keywords,omitempty"`
}

// DefaultImperativeConfig returns the embedded default configuration.
func DefaultImperativeConfig() *ImperativeConfig {
	data, err := defaultConfigFS.ReadFile("imperative_default.yaml")
	if err != nil {
		// Should never happen with embedded file; fall back to empty
		return &ImperativeConfig{Languages: map[string]LanguageRules{}}
	}
	var cfg ImperativeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return &ImperativeConfig{Languages: map[string]LanguageRules{}}
	}
	return &cfg
}
