// Package skill handles parsing of SKILL.md files, including YAML frontmatter
// extraction and body separation. It provides the core [Skill] type used by
// validation and scoring packages.
package skill

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/agent-ecosystem/skill-validator/util"
)

var _ yaml.Unmarshaler = (*AllowedTools)(nil)

// Frontmatter represents the parsed YAML frontmatter of a SKILL.md file.
type Frontmatter struct {
	Name          string `yaml:"name"`
	Description   string `yaml:"description"`
	License       string `yaml:"license"`
	Compatibility string `yaml:"compatibility"`
	// Metadata holds the spec-conforming entries of the metadata map: the
	// spec requires string keys and string values. It is populated from
	// RawFrontmatter in Load rather than unmarshaled directly so that
	// non-conforming entries (lists, maps, non-string scalars) surface as
	// structure validation errors instead of failing the YAML parse.
	Metadata     map[string]string `yaml:"-"`
	AllowedTools AllowedTools      `yaml:"allowed-tools"`
}

// AllowedTools handles the type ambiguity in the allowed-tools field.
// The spec defines it as a space-delimited string, but many skills use
// a YAML list instead. This type accepts both.
type AllowedTools struct {
	Value   string // normalized space-delimited string
	WasList bool   // true if the original YAML used a sequence
}

// UnmarshalYAML implements custom unmarshaling for AllowedTools to accept
// both string and list formats.
func (a *AllowedTools) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		a.Value = value.Value
		a.WasList = false
		return nil
	case yaml.SequenceNode:
		var items []string
		if err := value.Decode(&items); err != nil {
			return fmt.Errorf("decoding allowed-tools list: %w", err)
		}
		a.Value = strings.Join(items, " ")
		a.WasList = true
		return nil
	default:
		return fmt.Errorf("allowed-tools must be a string or list, got YAML node kind %d", value.Kind)
	}
}

// IsEmpty returns true if no allowed-tools value was specified.
func (a AllowedTools) IsEmpty() bool {
	return a.Value == ""
}

// Skill represents a parsed skill package.
type Skill struct {
	Dir            string
	Frontmatter    Frontmatter
	RawFrontmatter map[string]any
	Body           string
	RawContent     string
}

// knownFrontmatterFields lists the frontmatter field names defined by the
// skill spec. Fields not in this set trigger an "unrecognized field" warning.
var knownFrontmatterFields = map[string]bool{
	"name":          true,
	"description":   true,
	"license":       true,
	"compatibility": true,
	"metadata":      true,
	"allowed-tools": true,
}

// Load reads and parses a SKILL.md file from the given directory.
func Load(dir string) (*Skill, error) {
	path := filepath.Join(dir, "SKILL.md")
	data, err := util.SafeReadFile(dir, path)
	if err != nil {
		return nil, fmt.Errorf("reading SKILL.md: %w", err)
	}

	content := string(data)
	skill := &Skill{
		Dir:        dir,
		RawContent: content,
	}

	fm, body, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}

	skill.Body = body

	if fm != "" {
		if err := yaml.Unmarshal([]byte(fm), &skill.Frontmatter); err != nil {
			return nil, fmt.Errorf("parsing frontmatter YAML: %w", err)
		}
		if err := yaml.Unmarshal([]byte(fm), &skill.RawFrontmatter); err != nil {
			return nil, fmt.Errorf("parsing raw frontmatter: %w", err)
		}
		if md, ok := skill.RawFrontmatter["metadata"].(map[string]any); ok {
			skill.Frontmatter.Metadata = map[string]string{}
			for k, v := range md {
				if s, ok := v.(string); ok {
					skill.Frontmatter.Metadata[k] = s
				}
			}
		}
	}

	return skill, nil
}

// UnrecognizedFields returns frontmatter field names not in the spec.
func (s *Skill) UnrecognizedFields() []string {
	var unknown []string
	for k := range s.RawFrontmatter {
		if !knownFrontmatterFields[k] {
			unknown = append(unknown, k)
		}
	}
	return unknown
}

// splitFrontmatter separates YAML frontmatter (between --- delimiters) from the body.
func splitFrontmatter(content string) (frontmatter, body string, err error) {
	if !strings.HasPrefix(content, "---") {
		return "", content, nil
	}

	// Find the closing ---
	rest := content[3:]
	// Skip the newline after opening ---
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	// Handle empty frontmatter (closing --- immediately)
	if strings.HasPrefix(rest, "---") {
		frontmatter = ""
		body = rest[3:]
		if len(body) > 0 && body[0] == '\n' {
			body = body[1:]
		} else if len(body) > 1 && body[0] == '\r' && body[1] == '\n' {
			body = body[2:]
		}
		return frontmatter, body, nil
	}

	before, after, ok := strings.Cut(rest, "\n---")
	if !ok {
		return "", "", fmt.Errorf("unterminated frontmatter: missing closing ---")
	}

	frontmatter = strings.TrimRight(before, "\r")
	body = after // skip \n---
	// Strip leading newline from body
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	} else if len(body) > 1 && body[0] == '\r' && body[1] == '\n' {
		body = body[2:]
	}

	return frontmatter, body, nil
}
