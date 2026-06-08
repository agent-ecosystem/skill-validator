package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Backward compatibility tests (existing English behavior) ---

func TestAnalyze_EmptyContent(t *testing.T) {
	r := Analyze("")
	if r.WordCount != 0 {
		t.Errorf("expected 0 words, got %d", r.WordCount)
	}
}

func TestAnalyze_WordCount(t *testing.T) {
	r := Analyze("one two three four five")
	if r.WordCount != 5 {
		t.Errorf("expected 5 words, got %d", r.WordCount)
	}
}

func TestAnalyze_CodeBlocks(t *testing.T) {
	content := "Some text.\n\n```python\nprint('hello')\nprint('world')\n```\n\nMore text.\n\n```bash\necho hi\n```\n"
	r := Analyze(content)
	if r.CodeBlockCount != 2 {
		t.Errorf("expected 2 code blocks, got %d", r.CodeBlockCount)
	}
	if r.CodeBlockRatio <= 0 {
		t.Errorf("expected positive code block ratio, got %f", r.CodeBlockRatio)
	}
}

func TestAnalyze_CodeLanguages(t *testing.T) {
	content := "```python\ncode\n```\n\n```javascript\ncode\n```\n\n```python\ncode\n```\n"
	r := Analyze(content)
	if len(r.CodeLanguages) != 3 {
		t.Errorf("expected 3 code languages, got %d: %v", len(r.CodeLanguages), r.CodeLanguages)
	}
	if r.CodeLanguages[0] != "python" {
		t.Errorf("expected first language python, got %s", r.CodeLanguages[0])
	}
	if r.CodeLanguages[1] != "javascript" {
		t.Errorf("expected second language javascript, got %s", r.CodeLanguages[1])
	}
}

func TestAnalyze_ImperativeSentences(t *testing.T) {
	content := "Use the CLI tool. Run the tests. This is a description. Create a new file."
	r := Analyze(content)
	if r.ImperativeCount != 3 {
		t.Errorf("expected 3 imperative sentences, got %d", r.ImperativeCount)
	}
	if r.ImperativeRatio <= 0 {
		t.Errorf("expected positive imperative ratio, got %f", r.ImperativeRatio)
	}
}

func TestAnalyze_StrongMarkers(t *testing.T) {
	content := "You must always use this. Never do that. This is required."
	r := Analyze(content)
	if r.StrongMarkers < 4 {
		t.Errorf("expected at least 4 strong markers (must, always, never, required), got %d", r.StrongMarkers)
	}
}

func TestAnalyze_WeakMarkers(t *testing.T) {
	content := "You may consider this. It could work. It might be optional."
	r := Analyze(content)
	if r.WeakMarkers < 4 {
		t.Errorf("expected at least 4 weak markers (may, consider, could, might, optional), got %d", r.WeakMarkers)
	}
}

func TestAnalyze_InstructionSpecificity(t *testing.T) {
	content := "You must do this. You must always do that. Never skip it."
	r := Analyze(content)
	// All strong markers, no weak ones → specificity = 1.0
	if r.InstructionSpecificity != 1.0 {
		t.Errorf("expected specificity 1.0 with only strong markers, got %f", r.InstructionSpecificity)
	}
}

func TestAnalyze_Sections(t *testing.T) {
	content := "# Title\n\n## Section 1\n\nText.\n\n### Subsection\n\nMore text.\n\n## Section 2\n"
	r := Analyze(content)
	// H2+ headers: ## Section 1, ### Subsection, ## Section 2 = 3
	if r.SectionCount != 3 {
		t.Errorf("expected 3 sections, got %d", r.SectionCount)
	}
}

func TestAnalyze_ListItems(t *testing.T) {
	content := "- item 1\n- item 2\n* item 3\n1. numbered\n2. also numbered\n"
	r := Analyze(content)
	if r.ListItemCount != 5 {
		t.Errorf("expected 5 list items, got %d", r.ListItemCount)
	}
}

func TestAnalyze_InformationDensity(t *testing.T) {
	t.Run("with code blocks", func(t *testing.T) {
		content := "Use the tool.\n\n```bash\necho hello\n```\n\nRun the command. Build the project."
		r := Analyze(content)
		if r.InformationDensity <= 0 {
			t.Errorf("expected positive information density, got %f", r.InformationDensity)
		}
	})

	t.Run("without code blocks", func(t *testing.T) {
		// Prose-only skill with imperative sentences should not be penalized
		content := "Use the tool. Run the command. Build the project."
		r := Analyze(content)
		if r.CodeBlockCount != 0 {
			t.Errorf("expected 0 code blocks, got %d", r.CodeBlockCount)
		}
		if r.ImperativeRatio <= 0 {
			t.Fatalf("expected positive imperative ratio, got %f", r.ImperativeRatio)
		}
		// Without code blocks, information density should equal imperative ratio
		if r.InformationDensity != r.ImperativeRatio {
			t.Errorf("expected information density (%f) to equal imperative ratio (%f) when no code blocks",
				r.InformationDensity, r.ImperativeRatio)
		}
	})
}

func TestAnalyze_FullContent(t *testing.T) {
	content := `# My Skill

## Usage

Use the CLI to validate skills. You must always run tests before publishing.

` + "```bash\nskill-validator validate ./my-skill\n```" + `

## Configuration

Create a config file. Set the output format. You may consider using JSON.

- Step 1: Install
- Step 2: Configure
- Step 3: Run

Never skip validation. Ensure all checks pass.
`

	r := Analyze(content)

	if r.WordCount <= 0 {
		t.Error("expected positive word count")
	}
	if r.CodeBlockCount != 1 {
		t.Errorf("expected 1 code block, got %d", r.CodeBlockCount)
	}
	if r.SectionCount != 2 {
		t.Errorf("expected 2 sections, got %d", r.SectionCount)
	}
	if r.ListItemCount != 3 {
		t.Errorf("expected 3 list items, got %d", r.ListItemCount)
	}
	if r.StrongMarkers < 3 {
		t.Errorf("expected at least 3 strong markers, got %d", r.StrongMarkers)
	}
	if r.WeakMarkers < 1 {
		t.Errorf("expected at least 1 weak marker, got %d", r.WeakMarkers)
	}
	if r.InstructionSpecificity <= 0 || r.InstructionSpecificity > 1.0 {
		t.Errorf("expected specificity in (0, 1], got %f", r.InstructionSpecificity)
	}
}

// --- Multilingual tests ---

func TestAnalyze_ChineseImperativeSentences(t *testing.T) {
	content := "使用CLI工具。运行测试。这是一个描述。创建一个新文件。"
	r := Analyze(content)

	// "使用CLI工具" → starts with 使用 (imperative verb)
	// "运行测试" → starts with 运行 (imperative verb)
	// "这是一个描述" → not imperative
	// "创建一个新文件" → starts with 创建 (imperative verb)
	if r.ImperativeCount < 3 {
		t.Errorf("expected at least 3 imperative sentences, got %d", r.ImperativeCount)
	}
	if r.ImperativeRatio <= 0 {
		t.Errorf("expected positive imperative ratio, got %f", r.ImperativeRatio)
	}
}

func TestAnalyze_ChineseKeywordImperative(t *testing.T) {
	content := "请确保所有测试通过。务必按照规范操作。"
	r := Analyze(content)

	if r.ImperativeCount < 2 {
		t.Errorf("expected at least 2 imperative sentences (keyword match), got %d", r.ImperativeCount)
	}
}

func TestAnalyze_ChineseSentenceSplitting(t *testing.T) {
	content := "使用此工具。运行测试！确保正确？配置环境；这是说明。"
	r := Analyze(content)

	if r.SentenceCount < 4 {
		t.Errorf("expected at least 4 sentences (Chinese split), got %d", r.SentenceCount)
	}
}

func TestAnalyze_MixedChineseEnglish(t *testing.T) {
	content := "Use the CLI tool. 运行测试。Create a new file. 请确保代码质量。"
	r := Analyze(content)

	if r.SentenceCount < 3 {
		t.Errorf("expected at least 3 sentences in mixed content, got %d", r.SentenceCount)
	}
	// English imperatives: "Use the CLI tool", "Create a new file"
	// Chinese imperatives: "运行测试", "请确保代码质量" (keyword)
	if r.ImperativeCount < 3 {
		t.Errorf("expected at least 3 imperative sentences in mixed content, got %d", r.ImperativeCount)
	}
}

func TestAnalyzeWithConfig_CustomConfig(t *testing.T) {
	cfg := &ImperativeConfig{
		Languages: map[string]LanguageRules{
			"en": {
				Verbs: []string{"build", "deploy"},
			},
		},
	}

	content := "Build the project. Deploy to production. Use the tool."
	r := AnalyzeWithConfig(content, cfg)

	// Only "Build" and "Deploy" should be detected as imperative
	// "Use" is NOT in this custom config, but falls back to defaultImperativeVerbs
	if r.ImperativeCount < 2 {
		t.Errorf("expected at least 2 imperative sentences with custom config, got %d", r.ImperativeCount)
	}
}

func TestAnalyzeWithConfig_NilUsesDefault(t *testing.T) {
	content := "Use the CLI tool. Run the tests."
	r1 := Analyze(content)
	r2 := AnalyzeWithConfig(content, nil)

	if r1.ImperativeCount != r2.ImperativeCount {
		t.Errorf("Analyze and AnalyzeWithConfig(nil) should give same results: %d vs %d",
			r1.ImperativeCount, r2.ImperativeCount)
	}
}

func TestAnalyze_ChineseNonImperative(t *testing.T) {
	content := "这是一个描述。项目已完成。功能正常工作。"
	r := Analyze(content)

	if r.ImperativeCount != 0 {
		t.Errorf("expected 0 imperative sentences for non-imperative Chinese, got %d", r.ImperativeCount)
	}
}

// --- Config loading tests ---

func TestLoadImperativeConfig_DefaultEmbedded(t *testing.T) {
	cfg := DefaultImperativeConfig()
	if cfg == nil {
		t.Fatal("expected non-nil default config")
	}
	if len(cfg.Languages) == 0 {
		t.Error("expected at least one language in default config")
	}
	enRules, ok := cfg.Languages["en"]
	if !ok {
		t.Fatal("expected 'en' language in default config")
	}
	if len(enRules.Verbs) == 0 {
		t.Error("expected non-empty English verb list")
	}
	// Check backward compatibility: all original verbs should be present
	expectedVerbs := []string{
		"use", "run", "create", "add", "set", "install",
		"configure", "write", "read", "check", "verify", "make", "build", "test",
		"ensure", "include", "remove", "delete", "update", "call", "import",
		"export", "define", "implement", "return", "pass", "handle", "parse",
		"generate", "format", "validate", "convert", "follow", "apply", "start",
		"stop", "avoid", "keep", "do", "execute", "open", "close", "save",
		"load", "send", "receive",
	}

	verbSet := make(map[string]bool, len(enRules.Verbs))
	for _, v := range enRules.Verbs {
		verbSet[strings.ToLower(v)] = true
	}
	for _, v := range expectedVerbs {
		if !verbSet[v] {
			t.Errorf("expected verb %q in default English config", v)
		}
	}

	zhRules, ok := cfg.Languages["zh"]
	if !ok {
		t.Fatal("expected 'zh' language in default config")
	}
	if len(zhRules.Verbs) == 0 {
		t.Error("expected non-empty Chinese verb list")
	}
	if len(zhRules.Keywords) == 0 {
		t.Error("expected non-empty Chinese keyword list")
	}
}

func TestLoadImperativeConfig_FromFile(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "imperative.yaml")
	cfgContent := `languages:
  en:
    verbs:
      - custom_verb
      - another_verb
  zh:
    verbs:
      - 自定义
    keywords:
      - 请务必
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg := LoadImperativeConfig(cfgPath)
	if cfg == nil {
		t.Fatal("expected non-nil config from file")
	}

	enRules := cfg.Languages["en"]
	if len(enRules.Verbs) != 2 {
		t.Errorf("expected 2 English verbs from file, got %d", len(enRules.Verbs))
	}

	zhRules := cfg.Languages["zh"]
	if len(zhRules.Keywords) != 1 {
		t.Errorf("expected 1 Chinese keyword from file, got %d", len(zhRules.Keywords))
	}
}

func TestLoadImperativeConfig_InvalidPath(t *testing.T) {
	cfg := LoadImperativeConfig("/nonexistent/path/imperative.yaml")
	if cfg == nil {
		t.Fatal("expected fallback to default config for invalid path")
	}
	// Should fall back to defaults
	if len(cfg.Languages) == 0 {
		t.Error("expected default languages as fallback")
	}
}

func TestLoadImperativeConfig_EmptyPath(t *testing.T) {
	// Should not panic, returns default config when no file found
	cfg := LoadImperativeConfig("")
	if cfg == nil {
		t.Fatal("expected non-nil config with empty path")
	}
}

func TestHasChinese(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"Hello world", false},
		{"使用此工具", true},
		{"Use 使用 both", true},
		{"12345", false},
		{"", false},
	}
	for _, tt := range tests {
		got := hasChinese(tt.input)
		if got != tt.want {
			t.Errorf("hasChinese(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestSetImperativeConfig(t *testing.T) {
	// Save original to restore after test
	origCfg := DefaultImperativeConfig()
	defer SetImperativeConfig(origCfg)

	customCfg := &ImperativeConfig{
		Languages: map[string]LanguageRules{
			"en": {Verbs: []string{"deploy"}},
		},
	}
	SetImperativeConfig(customCfg)

	content := "Deploy the app. Use the tool."
	r := Analyze(content)

	if r.ImperativeCount < 1 {
		t.Errorf("expected at least 1 imperative (deploy), got %d", r.ImperativeCount)
	}
}

// --- Detector unit tests ---

func TestImperativeDetector_ChineseVerbs(t *testing.T) {
	d := newImperativeDetector(nil)

	tests := []struct {
		sentence string
		want     bool
	}{
		{"使用CLI工具进行验证", true},
		{"运行所有测试用例", true},
		{"创建新的配置文件", true},
		{"这是一个描述句子", false},
		{"项目已经完成了", false},
	}

	for _, tt := range tests {
		got := d.isImperative(tt.sentence)
		if got != tt.want {
			t.Errorf("isImperative(%q) = %v, want %v", tt.sentence, got, tt.want)
		}
	}
}

func TestImperativeDetector_ChineseKeywords(t *testing.T) {
	d := newImperativeDetector(nil)

	tests := []struct {
		sentence string
		want     bool
	}{
		{"请确保所有测试通过", true},
		{"务必按照规范操作", true},
		{"这只是一个普通句子", false},
	}

	for _, tt := range tests {
		got := d.isImperative(tt.sentence)
		if got != tt.want {
			t.Errorf("isImperative(%q) = %v, want %v", tt.sentence, got, tt.want)
		}
	}
}

func TestImperativeDetector_EnglishBackwardCompat(t *testing.T) {
	d := newImperativeDetector(nil)

	tests := []struct {
		sentence string
		want     bool
	}{
		{"Use the CLI tool", true},
		{"Run the tests", true},
		{"This is a description", false},
		{"Create a new file", true},
	}

	for _, tt := range tests {
		got := d.isImperative(tt.sentence)
		if got != tt.want {
			t.Errorf("isImperative(%q) = %v, want %v", tt.sentence, got, tt.want)
		}
	}
}

func TestImperativeDetector_MarkdownFormatting(t *testing.T) {
	d := newImperativeDetector(nil)

	tests := []struct {
		sentence string
		want     bool
	}{
		{"## Use the tool", true},
		{"- Run the tests", true},
		{"**Create** a file", false}, // "**Create" is not cleaned by leadingFormatPat
		{"  Set the value", true},
	}

	for _, tt := range tests {
		got := d.isImperative(tt.sentence)
		if got != tt.want {
			t.Errorf("isImperative(%q) = %v, want %v", tt.sentence, got, tt.want)
		}
	}
}

func TestSplitSentences_MixedContent(t *testing.T) {
	d := newImperativeDetector(nil)

	text := "Use the tool.运行测试。Create a file."
	sentences := d.splitSentences(text)

	if len(sentences) < 2 {
		t.Errorf("expected at least 2 sentences from mixed content, got %d: %v", len(sentences), sentences)
	}
}

func TestAnalyze_ChineseFullContent(t *testing.T) {
	content := `# 我的技能

## 使用说明

使用CLI工具验证技能。你必须始终在发布前运行测试。

` + "```bash\nskill-validator validate ./my-skill\n```" + `

## 配置

创建配置文件。设置输出格式。你可以考虑使用JSON。

- 步骤 1：安装
- 步骤 2：配置
- 步骤 3：运行

请确保所有检查通过。
`

	r := Analyze(content)

	if r.WordCount <= 0 {
		t.Error("expected positive word count")
	}
	if r.CodeBlockCount != 1 {
		t.Errorf("expected 1 code block, got %d", r.CodeBlockCount)
	}
	if r.SectionCount != 2 {
		t.Errorf("expected 2 sections, got %d", r.SectionCount)
	}
	// Chinese imperatives: "使用CLI工具验证技能", "创建配置文件", "设置输出格式", "请确保所有检查通过"
	if r.ImperativeCount < 3 {
		t.Errorf("expected at least 3 imperative sentences, got %d", r.ImperativeCount)
	}
}
