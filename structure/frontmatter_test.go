package structure

import (
	"strings"
	"testing"

	"github.com/agent-ecosystem/skill-validator/skill"
	"github.com/agent-ecosystem/skill-validator/types"
)

func makeSkill(dir, name, desc string) *skill.Skill {
	s := &skill.Skill{
		Dir: dir,
		Frontmatter: skill.Frontmatter{
			Name:        name,
			Description: desc,
		},
		RawFrontmatter: map[string]any{},
	}
	if name != "" {
		s.RawFrontmatter["name"] = name
	}
	if desc != "" {
		s.RawFrontmatter["description"] = desc
	}
	return s
}

func TestCheckFrontmatter_Name(t *testing.T) {
	t.Run("missing name", func(t *testing.T) {
		s := makeSkill("/tmp/my-skill", "", "A description")
		results := CheckFrontmatter(s, Options{})
		requireResult(t, results, types.Error, "name is required")
	})

	t.Run("valid name matching dir", func(t *testing.T) {
		s := makeSkill("/tmp/my-skill", "my-skill", "A description")
		results := CheckFrontmatter(s, Options{})
		requireResult(t, results, types.Pass, `name: "my-skill" (valid)`)
		requireNoResultContaining(t, results, types.Error, "name")
	})

	t.Run("name too long", func(t *testing.T) {
		longName := strings.Repeat("a", 65)
		s := makeSkill("/tmp/"+longName, longName, "A description")
		results := CheckFrontmatter(s, Options{})
		requireResult(t, results, types.Error, "name exceeds 64 characters (65)")
	})

	t.Run("name length counts characters, not bytes", func(t *testing.T) {
		// 40 two-byte runes: 80 bytes, 40 characters. The name pattern
		// rejects it, but the length check must not.
		name := strings.Repeat("\u00e9", 40)
		s := makeSkill("/tmp/"+name, name, "A description")
		results := CheckFrontmatter(s, Options{})
		requireNoResultContaining(t, results, types.Error, "exceeds 64 characters")
	})

	t.Run("name with uppercase", func(t *testing.T) {
		s := makeSkill("/tmp/My-Skill", "My-Skill", "A description")
		results := CheckFrontmatter(s, Options{})
		requireResultContaining(t, results, types.Error, "must be lowercase alphanumeric")
	})

	t.Run("name with consecutive hyphens", func(t *testing.T) {
		s := makeSkill("/tmp/my--skill", "my--skill", "A description")
		results := CheckFrontmatter(s, Options{})
		requireResultContaining(t, results, types.Error, "must be lowercase alphanumeric")
	})

	t.Run("name with leading hyphen", func(t *testing.T) {
		s := makeSkill("/tmp/-my-skill", "-my-skill", "A description")
		results := CheckFrontmatter(s, Options{})
		requireResultContaining(t, results, types.Error, "must be lowercase alphanumeric")
	})

	t.Run("name with trailing hyphen", func(t *testing.T) {
		s := makeSkill("/tmp/my-skill-", "my-skill-", "A description")
		results := CheckFrontmatter(s, Options{})
		requireResultContaining(t, results, types.Error, "must be lowercase alphanumeric")
	})

	t.Run("name does not match directory", func(t *testing.T) {
		s := makeSkill("/tmp/other-dir", "my-skill", "A description")
		results := CheckFrontmatter(s, Options{})
		requireResultContaining(t, results, types.Error, "name does not match directory name")
	})

	t.Run("single char name", func(t *testing.T) {
		s := makeSkill("/tmp/a", "a", "A description")
		results := CheckFrontmatter(s, Options{})
		requireResult(t, results, types.Pass, `name: "a" (valid)`)
	})

	t.Run("numeric name", func(t *testing.T) {
		s := makeSkill("/tmp/123", "123", "A description")
		results := CheckFrontmatter(s, Options{})
		requireResult(t, results, types.Pass, `name: "123" (valid)`)
	})
}

func TestCheckFrontmatter_Description(t *testing.T) {
	t.Run("missing description", func(t *testing.T) {
		s := makeSkill("/tmp/my-skill", "my-skill", "")
		results := CheckFrontmatter(s, Options{})
		requireResult(t, results, types.Error, "description is required")
	})

	t.Run("valid description", func(t *testing.T) {
		s := makeSkill("/tmp/my-skill", "my-skill", "A valid description")
		results := CheckFrontmatter(s, Options{})
		requireResultContaining(t, results, types.Pass, "description: (19 chars)")
	})

	t.Run("description too long", func(t *testing.T) {
		longDesc := strings.Repeat("x", 1025)
		s := makeSkill("/tmp/my-skill", "my-skill", longDesc)
		results := CheckFrontmatter(s, Options{})
		requireResult(t, results, types.Error, "description exceeds 1024 characters (1025)")
	})

	t.Run("multibyte description under the limit", func(t *testing.T) {
		// 342 CJK characters is 1026 UTF-8 bytes: over the limit if
		// counted in bytes, well under it in characters (#94).
		desc := strings.Repeat("\u6f22", 342)
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireNoResultContaining(t, results, types.Error, "description exceeds")
		requireResultContaining(t, results, types.Pass, "description: (342 chars)")
	})

	t.Run("multibyte description over the limit", func(t *testing.T) {
		desc := strings.Repeat("\u6f22", 1025)
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireResult(t, results, types.Error, "description exceeds 1024 characters (1025)")
	})

	t.Run("whitespace-only description", func(t *testing.T) {
		s := makeSkill("/tmp/my-skill", "my-skill", "   \t\n  ")
		results := CheckFrontmatter(s, Options{})
		requireResult(t, results, types.Error, "description must not be empty/whitespace-only")
	})
}

func TestCheckFrontmatter_KeywordStuffing(t *testing.T) {
	t.Run("normal description no warning", func(t *testing.T) {
		s := makeSkill("/tmp/my-skill", "my-skill", "A skill for building MongoDB vector search applications with best practices.")
		results := CheckFrontmatter(s, Options{})
		requireNoResultContaining(t, results, types.Warning, "keyword")
	})

	t.Run("description with a few quoted terms is fine", func(t *testing.T) {
		s := makeSkill("/tmp/my-skill", "my-skill", `Use when you see "vector search" or "embeddings" in a query.`)
		results := CheckFrontmatter(s, Options{})
		requireNoResultContaining(t, results, types.Warning, "keyword")
	})

	t.Run("description with many quoted strings and little prose", func(t *testing.T) {
		desc := `MongoDB vector search. Triggers on "vector search", "vector index", "$vectorSearch", "embedding", "semantic search", "RAG", "numCandidates".`
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireResultContaining(t, results, types.Warning, "quoted strings")
		requireResultContaining(t, results, types.Warning, "what the skill does and when to use it")
	})

	t.Run("prose with supplementary trigger list is fine", func(t *testing.T) {
		desc := `Azure Identity SDK for Python authentication. Use for DefaultAzureCredential, managed identity, service principals, and token caching. Triggers: "azure-identity", "DefaultAzureCredential", "authentication", "managed identity", "service principal", "credential".`
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireNoResultContaining(t, results, types.Warning, "quoted strings")
		requireNoResultContaining(t, results, types.Warning, "keyword")
	})

	t.Run("docx skill with trigger examples is fine", func(t *testing.T) {
		desc := `Use this skill whenever the user wants to create, read, edit, or manipulate Word documents (.docx files). Triggers include: any mention of "Word doc", "word document", ".docx", "resume", "cover letter", or requests to produce professional documents with formatting.`
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireNoResultContaining(t, results, types.Warning, "quoted strings")
		requireNoResultContaining(t, results, types.Warning, "keyword")
	})

	t.Run("comma-separated keyword list", func(t *testing.T) {
		desc := "MongoDB, Atlas, Vector Search, embeddings, RAG, retrieval, indexing, HNSW, quantization, similarity"
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireResultContaining(t, results, types.Warning, "comma-separated segments")
		requireResultContaining(t, results, types.Warning, "what the skill does and when to use it")
	})

	t.Run("legitimate list of features is fine", func(t *testing.T) {
		desc := "Helps with creating indexes, writing queries, and building applications."
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireNoResultContaining(t, results, types.Warning, "keyword")
		requireNoResultContaining(t, results, types.Warning, "comma-separated")
	})

	t.Run("only one warning when both heuristics match", func(t *testing.T) {
		desc := `Triggers on "a", "b", "c", "d", "e", "f", "g", "h", "i", "j".`
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireResultContaining(t, results, types.Warning, "quoted strings")
		requireNoResultContaining(t, results, types.Warning, "comma-separated segments")
	})

	t.Run("prose words equal to quote count is fine", func(t *testing.T) {
		// 5 quotes, 5 prose words (Manage identity tokens using SDK) — boundary: equal should pass
		desc := `Manage identity tokens using SDK. Triggers: "azure", "identity", "token", "credential", "auth".`
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireNoResultContaining(t, results, types.Warning, "quoted strings")
	})

	t.Run("all quoted strings no prose warns", func(t *testing.T) {
		desc := `"vector search" "embeddings" "RAG" "similarity" "indexing"`
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireResultContaining(t, results, types.Warning, "quoted strings")
	})

	t.Run("four quoted strings is fine", func(t *testing.T) {
		desc := `Use for "vector search", "embeddings", "RAG", and "similarity" queries.`
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireNoResultContaining(t, results, types.Warning, "quoted strings")
	})

	t.Run("bare keyword list with some quoted terms still warns", func(t *testing.T) {
		desc := `MongoDB, Atlas, "Vector Search", embeddings, RAG, retrieval, indexing, HNSW, "quantization", similarity`
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireResultContaining(t, results, types.Warning, "comma-separated segments")
	})

	t.Run("segments below threshold after empty filtering is fine", func(t *testing.T) {
		// Raw commas from quoted strings create empty segments; after filtering, only 4 real segments remain
		desc := `Use this skill for Python authentication and credential management. Triggers: "azure", "identity", "token", "credential", "auth", "login".`
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireNoResultContaining(t, results, types.Warning, "comma-separated")
	})

	t.Run("many commas but long segments is fine", func(t *testing.T) {
		desc := "Use when creating vector indexes for search, writing complex aggregation queries with multiple stages, building RAG applications with retrieval patterns, implementing hybrid search with rank fusion, storing AI agent memory in collections, optimizing search performance with explain plans, configuring HNSW index parameters for your workload, tuning numCandidates for recall versus latency tradeoffs"
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireNoResultContaining(t, results, types.Warning, "comma-separated segments")
	})

	t.Run("multi-sentence prose with inline lists is fine (issue #26)", func(t *testing.T) {
		desc := "Manages MongoDB Atlas Stream Processing (ASP) workflows. Handles workspace provisioning, data source/sink connections, processor lifecycle operations, debugging diagnostics, and tier sizing. Supports Kafka, Atlas clusters, S3, HTTPS, and Lambda integrations for streaming data workloads and event processing. NOT for general MongoDB queries or Atlas cluster management. Requires MongoDB MCP Server with Atlas API credentials."
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireNoResultContaining(t, results, types.Warning, "keyword")
		requireNoResultContaining(t, results, types.Warning, "comma-separated")
	})

	t.Run("prose followed by keyword dump still warns", func(t *testing.T) {
		desc := "Manages MongoDB workflows. MongoDB, Atlas, Vector Search, embeddings, RAG, retrieval, indexing, HNSW"
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireResultContaining(t, results, types.Warning, "comma-separated segments")
	})

	t.Run("prose with inline enumeration is fine (issue #71)", func(t *testing.T) {
		desc := "Helps agents write and edit interface copy (microcopy) for digital products — buttons, labels, error messages, forms, onboarding flows, empty states, and help text. Use this skill whenever you need to produce or improve any text that appears in an app, website, or software UI. It applies four core quality standards (purposeful, concise, conversational, and clear) and ships with accessibility guidelines, research-backed readability benchmarks, error-message patterns, tone adaptation frameworks, and fillable templates."
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireNoResultContaining(t, results, types.Warning, "keyword")
		requireNoResultContaining(t, results, types.Warning, "comma-separated")
	})

	t.Run("description with abbreviations splits correctly", func(t *testing.T) {
		desc := "Use for e.g. vector search and embedding workflows. Supports multiple backends, distributed indexing, and query optimization."
		s := makeSkill("/tmp/my-skill", "my-skill", desc)
		results := CheckFrontmatter(s, Options{})
		requireNoResultContaining(t, results, types.Warning, "keyword")
		requireNoResultContaining(t, results, types.Warning, "comma-separated")
	})
}

func TestSplitSentences(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single sentence",
			input:    "Hello world.",
			expected: []string{"Hello world."},
		},
		{
			name:  "two sentences",
			input: "Hello world. Goodbye world.",
			expected: []string{
				"Hello world.",
				"Goodbye world.",
			},
		},
		{
			name:  "abbreviation e.g. not split",
			input: "Use for e.g. vector search and embeddings. Next sentence here.",
			expected: []string{
				"Use for e.g. vector search and embeddings.",
				"Next sentence here.",
			},
		},
		{
			name:  "abbreviation i.e. not split",
			input: "Works with i.e. the primary backend. Supports caching.",
			expected: []string{
				"Works with i.e. the primary backend.",
				"Supports caching.",
			},
		},
		{
			name:  "version number not split",
			input: "Requires Node.js v18.0 or higher. Also supports Deno.",
			expected: []string{
				"Requires Node.js v18.0 or higher.",
				"Also supports Deno.",
			},
		},
		{
			name:  "etc. is a sentence boundary",
			input: "Handles auth, storage, etc. Supports caching.",
			expected: []string{
				"Handles auth, storage, etc.",
				"Supports caching.",
			},
		},
		{
			name:     "no trailing period",
			input:    "Hello world",
			expected: []string{"Hello world"},
		},
		{
			name:  "exclamation mark boundary",
			input: "This is great! Use it now.",
			expected: []string{
				"This is great!",
				"Use it now.",
			},
		},
		{
			name:  "question mark boundary",
			input: "Need vector search? Try this skill.",
			expected: []string{
				"Need vector search?",
				"Try this skill.",
			},
		},
		{
			name:     "no split when lowercase follows period",
			input:    "Use for auth. works with all backends.",
			expected: []string{"Use for auth. works with all backends."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitSentences(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("splitSentences(%q) = %v (len %d), want %v (len %d)",
					tt.input, got, len(got), tt.expected, len(tt.expected))
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("sentence[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestCheckFrontmatter_Compatibility(t *testing.T) {
	t.Run("valid compatibility", func(t *testing.T) {
		s := makeSkill("/tmp/my-skill", "my-skill", "desc")
		s.Frontmatter.Compatibility = "Works with GPT-4"
		results := CheckFrontmatter(s, Options{})
		requireResultContaining(t, results, types.Pass, "compatibility:")
	})

	t.Run("compatibility too long", func(t *testing.T) {
		s := makeSkill("/tmp/my-skill", "my-skill", "desc")
		s.Frontmatter.Compatibility = strings.Repeat("x", 501)
		results := CheckFrontmatter(s, Options{})
		requireResult(t, results, types.Error, "compatibility exceeds 500 characters (501)")
	})

	t.Run("multibyte compatibility under the limit", func(t *testing.T) {
		// 400 CJK characters is 1200 bytes but only 400 characters.
		s := makeSkill("/tmp/my-skill", "my-skill", "desc")
		s.Frontmatter.Compatibility = strings.Repeat("\u6f22", 400)
		results := CheckFrontmatter(s, Options{})
		requireNoResultContaining(t, results, types.Error, "compatibility exceeds")
		requireResultContaining(t, results, types.Pass, "compatibility: (400 chars)")
	})

	t.Run("multibyte compatibility over the limit", func(t *testing.T) {
		s := makeSkill("/tmp/my-skill", "my-skill", "desc")
		s.Frontmatter.Compatibility = strings.Repeat("\u6f22", 501)
		results := CheckFrontmatter(s, Options{})
		requireResult(t, results, types.Error, "compatibility exceeds 500 characters (501)")
	})
}

func TestCheckFrontmatter_Metadata(t *testing.T) {
	t.Run("valid string metadata", func(t *testing.T) {
		s := makeSkill("/tmp/my-skill", "my-skill", "desc")
		s.RawFrontmatter["metadata"] = map[string]any{
			"author":  "alice",
			"version": "1.0",
		}
		results := CheckFrontmatter(s, Options{})
		requireResultContaining(t, results, types.Pass, "metadata: (2 entries)")
	})

	t.Run("metadata with non-string value", func(t *testing.T) {
		s := makeSkill("/tmp/my-skill", "my-skill", "desc")
		s.RawFrontmatter["metadata"] = map[string]any{
			"count": 42,
		}
		results := CheckFrontmatter(s, Options{})
		requireResultContaining(t, results, types.Error, "metadata[\"count\"] value must be a string")
	})

	t.Run("metadata not a map", func(t *testing.T) {
		s := makeSkill("/tmp/my-skill", "my-skill", "desc")
		s.RawFrontmatter["metadata"] = "not a map"
		results := CheckFrontmatter(s, Options{})
		requireResult(t, results, types.Error, "metadata must be a map of string keys to string values")
	})
}

func TestCheckFrontmatter_OptionalFields(t *testing.T) {
	t.Run("license present", func(t *testing.T) {
		s := makeSkill("/tmp/my-skill", "my-skill", "desc")
		s.Frontmatter.License = "MIT"
		results := CheckFrontmatter(s, Options{})
		requireResult(t, results, types.Pass, `license: "MIT"`)
	})

	t.Run("allowed-tools string", func(t *testing.T) {
		s := makeSkill("/tmp/my-skill", "my-skill", "desc")
		s.Frontmatter.AllowedTools = skill.AllowedTools{Value: "Bash Read", WasList: false}
		results := CheckFrontmatter(s, Options{})
		requireResult(t, results, types.Pass, `allowed-tools: "Bash Read"`)
		requireNoResultContaining(t, results, types.Info, "YAML list")
	})

	t.Run("allowed-tools list emits info", func(t *testing.T) {
		s := makeSkill("/tmp/my-skill", "my-skill", "desc")
		s.Frontmatter.AllowedTools = skill.AllowedTools{Value: "Read Bash Grep", WasList: true}
		results := CheckFrontmatter(s, Options{})
		requireResult(t, results, types.Pass, `allowed-tools: "Read Bash Grep"`)
		requireResultContaining(t, results, types.Info, "YAML list")
		requireResultContaining(t, results, types.Info, "space-delimited string")
	})
}

func TestCheckFrontmatter_UnrecognizedFields(t *testing.T) {
	s := makeSkill("/tmp/my-skill", "my-skill", "desc")
	s.RawFrontmatter["custom"] = "value"
	results := CheckFrontmatter(s, Options{})
	requireResult(t, results, types.Warning, `unrecognized field: "custom"`)
}

func TestCheckFrontmatter_AllowExtraFrontmatter(t *testing.T) {
	s := makeSkill("/tmp/my-skill", "my-skill", "desc")
	s.RawFrontmatter["user-invokable"] = true
	s.RawFrontmatter["custom-field"] = "value"

	// Without the flag, unrecognized fields produce warnings
	results := CheckFrontmatter(s, Options{})
	requireResultContaining(t, results, types.Warning, `unrecognized field:`)

	// With the flag, no warnings for unrecognized fields
	results = CheckFrontmatter(s, Options{AllowExtraFrontmatter: true})
	for _, r := range results {
		if r.Level == types.Warning && strings.Contains(r.Message, "unrecognized field") {
			t.Errorf("expected no unrecognized field warnings with AllowExtraFrontmatter, got: %s", r.Message)
		}
	}
}
