package structure

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent-ecosystem/skill-validator/types"
	"github.com/agent-ecosystem/skill-validator/util"
)

// writeSparseFile creates a file of the given size without writing its
// bytes, so an over-limit fixture costs no disk (issue #87).
func writeSparseFile(t *testing.T, dir, rel string, size int64) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCheckTokens_TruncatedFile(t *testing.T) {
	saved := maxTokenizedFileBytes
	maxTokenizedFileBytes = 32
	t.Cleanup(func() { maxTokenizedFileBytes = saved })

	dir := t.TempDir()
	content := strings.Repeat("word ", 40) // 200 bytes
	writeFile(t, dir, "references/big.md", content)
	writeFile(t, dir, "assets/template.md", content)
	writeFile(t, dir, "references/small.md", "tiny")

	results, counts, _ := CheckTokens(dir, "Body.", Options{})

	requireResultContaining(t, results, types.Warning,
		"references/big.md is larger than 32 bytes; its token count covers only the first 32 bytes")
	requireResultContaining(t, results, types.Warning,
		"assets/template.md is larger than 32 bytes")
	requireNoResultContaining(t, results, types.Warning, "references/small.md is larger")

	enc, err := getEncoder()
	if err != nil {
		t.Fatal(err)
	}
	prefixTokens, _, _ := enc.Encode(content[:32])
	for _, c := range counts {
		switch c.File {
		case "references/big.md", "assets/template.md":
			if !c.Truncated {
				t.Errorf("%s: Truncated = false, want true", c.File)
			}
			if c.Tokens != len(prefixTokens) {
				t.Errorf("%s: Tokens = %d, want %d (count of the 32-byte prefix)", c.File, c.Tokens, len(prefixTokens))
			}
		case "references/small.md":
			if c.Truncated {
				t.Errorf("%s: Truncated = true, want false", c.File)
			}
		}
	}
}

func TestCheckMarkdown_OversizedReferenceSkipped(t *testing.T) {
	dir := t.TempDir()
	writeSparseFile(t, dir, "references/big.md", util.MaxSkillFileBytes+1)
	writeFile(t, dir, "references/broken.md", "# Ref\n```\nunclosed")

	results := CheckMarkdown(dir, "Clean body.")

	requireResultContaining(t, results, types.Warning,
		"references/big.md is larger than 8 MiB; skipped the unclosed code fence check")
	requireNoResultContaining(t, results, types.Error, "references/big.md")
	// The other files are still checked.
	requireResultContaining(t, results, types.Error, "references/broken.md has an unclosed code fence")
}

func TestCheckOrphanFiles_OversizedFileNotScanned(t *testing.T) {
	dir := t.TempDir()
	writeSparseFile(t, dir, "references/big.md", util.MaxSkillFileBytes+1)
	writeFile(t, dir, "references/leaf.md", "leaf content")

	// SKILL.md reaches big.md; whatever big.md links to is unknowable.
	results := CheckOrphanFiles(dir, "See references/big.md", Options{})

	requireResultContaining(t, results, types.Warning,
		"references/big.md is larger than 8 MiB and was not scanned for references")
	requireResultContaining(t, results, types.Warning,
		"potentially unreferenced file: references/leaf.md")
	requireNoResultContaining(t, results, types.Warning, "potentially unreferenced file: references/big.md")

	// The unscanned warning precedes the orphan warnings it qualifies.
	unscannedAt, orphanAt := -1, -1
	for i, r := range results {
		switch {
		case strings.Contains(r.Message, "was not scanned"):
			unscannedAt = i
		case strings.Contains(r.Message, "potentially unreferenced"):
			orphanAt = i
		}
	}
	if unscannedAt < 0 || orphanAt < 0 || unscannedAt > orphanAt {
		t.Errorf("unscanned warning at %d, orphan warning at %d; want unscanned first", unscannedAt, orphanAt)
	}
}
