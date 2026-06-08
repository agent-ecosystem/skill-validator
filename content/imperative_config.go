package content

import (
	"embed"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

//go:embed imperative_default.yaml
var defaultConfigFS embed.FS

// ImperativeConfig holds the multilingual imperative detection configuration.
type ImperativeConfig struct {
	// Languages maps a language tag (e.g. "en", "zh") to its detection rules.
	Languages map[string]LanguageRules `yaml:"languages"`

	// SentenceSplitPattern is an optional global regex override for sentence
	// splitting.  Per-language split patterns take precedence when set.
	SentenceSplitPattern string `yaml:"sentence_split_pattern,omitempty"`
}

// LanguageRules defines imperative detection rules for one language.
type LanguageRules struct {
	// Verbs is a list of imperative verbs/phrases to match at sentence start.
	Verbs []string `yaml:"verbs"`

	// Keywords is a list of imperative keywords/phrases matched anywhere in
	// the sentence (useful for languages like Chinese where imperatives don't
	// always start with a verb).
	Keywords []string `yaml:"keywords,omitempty"`

	// SentenceSplitPattern is a regex that overrides the default sentence
	// splitter for this language.
	SentenceSplitPattern string `yaml:"sentence_split_pattern,omitempty"`
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

// LoadImperativeConfig loads the imperative config from the given path.
// If path is empty, it tries the standard lookup order:
//  1. ./imperative.yaml (current working directory)
//  2. ~/.config/skill-validator/imperative.yaml
//  3. Embedded defaults
func LoadImperativeConfig(path string) *ImperativeConfig {
	if path != "" {
		return loadConfigFrom(path)
	}

	// Try CWD
	cwdPath := filepath.Join(".", "imperative.yaml")
	if data, err := os.ReadFile(cwdPath); err == nil {
		return parseConfig(data)
	}

	// Try user config dir
	home, err := os.UserHomeDir()
	if err == nil {
		userPath := filepath.Join(home, ".config", "skill-validator", "imperative.yaml")
		if data, err := os.ReadFile(userPath); err == nil {
			return parseConfig(data)
		}
	}

	// Fall back to embedded defaults
	return DefaultImperativeConfig()
}

func loadConfigFrom(path string) *ImperativeConfig {
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultImperativeConfig()
	}
	return parseConfig(data)
}

func parseConfig(data []byte) *ImperativeConfig {
	var cfg ImperativeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return DefaultImperativeConfig()
	}
	return &cfg
}
