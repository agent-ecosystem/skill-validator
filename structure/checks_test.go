package structure

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agent-ecosystem/skill-validator/types"
)

func TestCheckStructure(t *testing.T) {
	t.Run("missing SKILL.md", func(t *testing.T) {
		dir := t.TempDir()
		results := CheckStructure(dir, Options{})
		requireResult(t, results, types.Error, "SKILL.md not found")
	})

	t.Run("only SKILL.md", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "---\nname: test\n---\n")
		results := CheckStructure(dir, Options{})
		requireResult(t, results, types.Pass, "SKILL.md found")
		requireNoLevel(t, results, types.Error)
		requireNoLevel(t, results, types.Warning)
	})

	t.Run("recognized directories", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		if err := os.MkdirAll(filepath.Join(dir, "scripts"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "references"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
			t.Fatal(err)
		}
		results := CheckStructure(dir, Options{})
		requireResult(t, results, types.Pass, "SKILL.md found")
		requireNoLevel(t, results, types.Warning)
	})

	t.Run("unknown directory empty", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		if err := os.MkdirAll(filepath.Join(dir, "extras"), 0o755); err != nil {
			t.Fatal(err)
		}
		results := CheckStructure(dir, Options{})
		requireResult(t, results, types.Warning, "unknown directory: extras/")
	})

	t.Run("unknown directory with files suggests both dirs", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "rules/rule1.md", "rule one")
		writeFile(t, dir, "rules/rule2.md", "rule two")
		writeFile(t, dir, "rules/rule3.md", "rule three")
		results := CheckStructure(dir, Options{})
		requireResultContaining(t, results, types.Warning, "unknown directory: rules/ (contains 3 files)")
		requireResultContaining(t, results, types.Warning, "won't discover these files")
		requireResultContaining(t, results, types.Warning, "should this be references/ or assets/?")
	})

	t.Run("unknown directory hint omits references when it exists", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		if err := os.MkdirAll(filepath.Join(dir, "references"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, dir, "extras/file.md", "content")
		results := CheckStructure(dir, Options{})
		requireResultContaining(t, results, types.Warning, "should this be assets/?")
		requireNoResultContaining(t, results, types.Warning, "references/")
	})

	t.Run("unknown directory hint omits assets when it exists", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, dir, "extras/file.md", "content")
		results := CheckStructure(dir, Options{})
		requireResultContaining(t, results, types.Warning, "should this be references/?")
		requireNoResultContaining(t, results, types.Warning, "assets/")
	})

	t.Run("unknown directory hint omitted when both exist", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		if err := os.MkdirAll(filepath.Join(dir, "references"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, dir, "extras/file.md", "content")
		results := CheckStructure(dir, Options{})
		requireResultContaining(t, results, types.Warning, "won't discover these files")
		requireNoResultContaining(t, results, types.Warning, "should this be")
	})

	t.Run("unknown directory with hidden files excluded from count", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "extras/visible.md", "content")
		writeFile(t, dir, "extras/.hidden", "secret")
		results := CheckStructure(dir, Options{})
		requireResultContaining(t, results, types.Warning, "unknown directory: extras/ (contains 1 file)")
	})

	t.Run("AGENTS.md has specific warning", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "AGENTS.md", "agent config")
		results := CheckStructure(dir, Options{})
		requireResultContaining(t, results, types.Warning, "repo-level agent configuration")
		requireResultContaining(t, results, types.Warning, "move it outside the skill directory")
	})

	t.Run("known extraneous file README.md", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "README.md", "readme")
		results := CheckStructure(dir, Options{})
		requireResultContaining(t, results, types.Warning, "README.md is not needed in a skill")
		requireResultContaining(t, results, types.Warning, "Anthropic best practices")
	})

	t.Run("known extraneous file CHANGELOG.md", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "CHANGELOG.md", "changes")
		results := CheckStructure(dir, Options{})
		requireResultContaining(t, results, types.Warning, "CHANGELOG.md is not needed in a skill")
	})

	t.Run("known extraneous file LICENSE", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "LICENSE", "mit")
		results := CheckStructure(dir, Options{})
		requireResultContaining(t, results, types.Warning, "LICENSE is not needed in a skill")
	})

	t.Run("unknown file at root", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "notes.txt", "some notes")
		results := CheckStructure(dir, Options{})
		requireResultContaining(t, results, types.Warning, "unexpected file at root: notes.txt")
		requireResultContaining(t, results, types.Warning, "move it into references/ or assets/")
		requireResultContaining(t, results, types.Warning, "otherwise remove it")
	})

	t.Run("deep nesting", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		if err := os.MkdirAll(filepath.Join(dir, "references", "subdir"), 0o755); err != nil {
			t.Fatal(err)
		}
		results := CheckStructure(dir, Options{})
		requireResult(t, results, types.Warning, "deep nesting detected: references/subdir/")
	})

	t.Run("allow-nested-paths suppresses only the selected subtree", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "assets/components/button/include.md", "button")
		writeFile(t, dir, "assets/components-extra/card/include.md", "card")

		results := CheckStructure(dir, Options{AllowNestedPaths: []string{"assets/components"}})

		requireNoResultContaining(t, results, types.Warning, "deep nesting detected: assets/components/")
		requireResult(t, results, types.Warning, "deep nesting detected: assets/components-extra/")
		requireResultContaining(t, results, types.Info, "deep nesting allowed: assets/components/")
	})

	t.Run("allow-nested-paths accepts normalized Windows separators", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "assets/components/button/include.md", "button")

		results := CheckStructure(dir, Options{AllowNestedPaths: []string{`assets\components`}})

		requireNoResultContaining(t, results, types.Warning, "deep nesting detected: assets/components/")
		requireResultContaining(t, results, types.Info, "deep nesting allowed: assets/components/")
	})

	t.Run("hidden files and dirs are skipped", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, ".hidden", "secret")
		if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		results := CheckStructure(dir, Options{})
		requireResult(t, results, types.Pass, "SKILL.md found")
		requireNoLevel(t, results, types.Warning)
	})

	t.Run("hidden dirs inside recognized dirs are skipped", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		if err := os.MkdirAll(filepath.Join(dir, "references", ".hidden"), 0o755); err != nil {
			t.Fatal(err)
		}
		results := CheckStructure(dir, Options{})
		requireNoLevel(t, results, types.Warning)
	})

	t.Run("allow-flat-layouts suppresses root file warnings", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "README.md", "readme")
		writeFile(t, dir, "notes.txt", "notes")
		writeFile(t, dir, "AGENTS.md", "agent config")
		results := CheckStructure(dir, Options{AllowFlatLayouts: true})
		requireResult(t, results, types.Pass, "SKILL.md found")
		requireNoLevel(t, results, types.Warning)
	})

	t.Run("allow-flat-layouts still warns on unknown directories", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "extras/file.md", "content")
		results := CheckStructure(dir, Options{AllowFlatLayouts: true})
		requireResultContaining(t, results, types.Warning, "unknown directory: extras/")
	})

	t.Run("allow-dirs suppresses warning for allowed directory", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "evals/evals.json", `{"tests": []}`)
		results := CheckStructure(dir, Options{AllowDirs: []string{"evals"}})
		requireResult(t, results, types.Pass, "SKILL.md found")
		requireNoLevel(t, results, types.Warning)
	})

	t.Run("allow-dirs with multiple directories", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "evals/evals.json", `{"tests": []}`)
		writeFile(t, dir, "testing/test1.md", "test content")
		results := CheckStructure(dir, Options{AllowDirs: []string{"evals", "testing"}})
		requireResult(t, results, types.Pass, "SKILL.md found")
		requireNoLevel(t, results, types.Warning)
	})

	t.Run("allow-dirs partial allows still warn for non-allowed dirs", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "evals/evals.json", `{"tests": []}`)
		writeFile(t, dir, "extras/file.md", "content")
		results := CheckStructure(dir, Options{AllowDirs: []string{"evals"}})
		requireNoResultContaining(t, results, types.Warning, "evals/")
		requireResultContaining(t, results, types.Warning, "unknown directory: extras/")
	})

	t.Run("allow-dirs silently accepts already-recognized directory", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "scripts/setup.sh", "#!/bin/bash")
		results := CheckStructure(dir, Options{AllowDirs: []string{"scripts"}})
		requireResult(t, results, types.Pass, "SKILL.md found")
		requireNoLevel(t, results, types.Warning)
	})

	t.Run("allow-dirs exempt from deep nesting checks", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "evals/files/test1.txt", "test input")
		results := CheckStructure(dir, Options{AllowDirs: []string{"evals"}})
		requireResult(t, results, types.Pass, "SKILL.md found")
		requireNoResultContaining(t, results, types.Warning, "deep nesting")
	})

	t.Run("allow-dirs does not exempt recognized dirs from deep nesting", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "evals/files/test1.txt", "test input")
		if err := os.MkdirAll(filepath.Join(dir, "references", "subdir"), 0o755); err != nil {
			t.Fatal(err)
		}
		results := CheckStructure(dir, Options{AllowDirs: []string{"evals"}})
		requireNoResultContaining(t, results, types.Warning, "evals/")
		requireResult(t, results, types.Warning, "deep nesting detected: references/subdir/")
	})

	t.Run("allow-dirs with allow-flat-layouts", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "README.md", "readme")
		writeFile(t, dir, "notes.txt", "notes")
		writeFile(t, dir, "evals/evals.json", `{"tests": []}`)
		results := CheckStructure(dir, Options{AllowFlatLayouts: true, AllowDirs: []string{"evals"}})
		requireResult(t, results, types.Pass, "SKILL.md found")
		requireNoLevel(t, results, types.Warning)
	})

	t.Run("allow-dirs with allow-flat-layouts still warns on non-allowed dirs", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "README.md", "readme")
		writeFile(t, dir, "extras/file.md", "content")
		writeFile(t, dir, "evals/evals.json", `{"tests": []}`)
		results := CheckStructure(dir, Options{AllowFlatLayouts: true, AllowDirs: []string{"evals"}})
		requireNoLevel(t, results, types.Error)
		requireNoResultContaining(t, results, types.Warning, "README.md")
		requireNoResultContaining(t, results, types.Warning, "evals/")
		requireResultContaining(t, results, types.Warning, "unknown directory: extras/")
	})

	t.Run("allow-dirs hint still shown for non-allowed unknown dirs", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		writeFile(t, dir, "evals/evals.json", `{"tests": []}`)
		writeFile(t, dir, "extras/file.md", "content")
		results := CheckStructure(dir, Options{AllowDirs: []string{"evals"}})
		requireResultContaining(t, results, types.Warning, "should this be references/ or assets/?")
	})
}
