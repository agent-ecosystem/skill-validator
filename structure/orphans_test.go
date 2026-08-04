package structure

import (
	"strings"
	"testing"

	"github.com/agent-ecosystem/skill-validator/types"
)

func TestCheckOrphanFiles(t *testing.T) {
	t.Run("all files referenced from SKILL.md", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "references/guide.md", "guide content")
		writeFile(t, dir, "scripts/setup.sh", "#!/bin/bash")
		writeFile(t, dir, "assets/logo.png", "fake image")

		body := "See references/guide.md and scripts/setup.sh and assets/logo.png"
		results := CheckOrphanFiles(dir, body, Options{})

		requireResult(t, results, types.Pass, "all files in scripts/ are referenced")
		requireResult(t, results, types.Pass, "all files in references/ are referenced")
		requireResult(t, results, types.Pass, "all files in assets/ are referenced")
		requireNoLevel(t, results, types.Warning)
	})

	t.Run("orphan in references", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "references/guide.md", "guide content")
		writeFile(t, dir, "references/unused.md", "unused content")

		body := "See references/guide.md for details."
		results := CheckOrphanFiles(dir, body, Options{})

		requireResultContaining(t, results, types.Warning, "potentially unreferenced file: references/unused.md")
	})

	// Issue #85: a sibling file whose extensionless name is an ordinary word
	// ("lint") must not be treated as referenced by prose in the same
	// directory. The unreferenced file gets an orphan warning, not a
	// misleading missing-extension warning.
	t.Run("prose word matching sibling filename is not a reference", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "references/checklist.md", "Run the lint step before committing. Linting catches most problems.")
		writeFile(t, dir, "references/lint.yml", "name: lint\non: [push]\n")

		body := "See references/checklist.md for details."
		results := CheckOrphanFiles(dir, body, Options{})

		for _, r := range results {
			if strings.Contains(r.Message, "referenced without its extension") {
				t.Errorf("unexpected missing-extension warning: %s", r.Message)
			}
		}
		requireResultContaining(t, results, types.Warning, "potentially unreferenced file: references/lint.yml")
	})

	t.Run("orphan in scripts", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "scripts/setup.sh", "#!/bin/bash")

		body := "No references to scripts here."
		results := CheckOrphanFiles(dir, body, Options{})

		requireResultContaining(t, results, types.Warning, "potentially unreferenced file: scripts/setup.sh")
	})

	t.Run("empty directories produce no results", func(t *testing.T) {
		dir := t.TempDir()
		// No files at all
		results := CheckOrphanFiles(dir, "some body", Options{})
		if len(results) != 0 {
			t.Errorf("expected 0 results for empty dirs, got %d", len(results))
		}
	})

	t.Run("no recognized directories", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "other/file.md", "content")

		results := CheckOrphanFiles(dir, "some body", Options{})
		if len(results) != 0 {
			t.Errorf("expected 0 results for unrecognized dirs, got %d", len(results))
		}
	})

	t.Run("binary file referenced but not scanned", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "assets/logo.png", "fake binary with references/secret.md inside")
		writeFile(t, dir, "references/secret.md", "secret content")

		body := "See assets/logo.png for the logo."
		results := CheckOrphanFiles(dir, body, Options{})

		// logo.png is reached (referenced from body) but not scanned for further refs
		// so references/secret.md should be an orphan
		requireResultContaining(t, results, types.Warning, "potentially unreferenced file: references/secret.md")
		requireNoResultContaining(t, results, types.Warning, "assets/logo.png")
	})

	t.Run("directory-relative reference from referenced file", func(t *testing.T) {
		dir := t.TempDir()
		// references/guide.md references images/diagram.png using a path
		// relative to its own directory, not the skill root.
		writeFile(t, dir, "references/guide.md", "See ![diagram](images/diagram.png) for details.")
		writeFile(t, dir, "references/images/diagram.png", "fake image")

		body := "Read the [guide](references/guide.md)."
		results := CheckOrphanFiles(dir, body, Options{})

		// The image should be reached (indirectly via guide.md), not flagged as orphan
		requireNoResultContaining(t, results, types.Warning, "references/images/diagram.png")
		requireResult(t, results, types.Pass, "all files in references/ are referenced")
	})

	t.Run("root-level file bridges SKILL.md to scripts", func(t *testing.T) {
		dir := t.TempDir()
		// SKILL.md mentions FORMS.md (root-level), which mentions the script
		writeFile(t, dir, "FORMS.md", "Run scripts/fill_form.py to fill the form.")
		writeFile(t, dir, "scripts/fill_form.py", "#!/usr/bin/env python3")

		body := "For form filling, read FORMS.md and follow its instructions."
		results := CheckOrphanFiles(dir, body, Options{})

		requireNoResultContaining(t, results, types.Warning, "scripts/fill_form.py")
		requireResult(t, results, types.Pass, "all files in scripts/ are referenced")
	})

	t.Run("package.json bridges SKILL.md to scripts when referenced", func(t *testing.T) {
		dir := t.TempDir()
		// SKILL.md mentions package.json, which maps to the script
		writeFile(t, dir, "package.json", `{"scripts":{"validate":"node scripts/validate.js"}}`)
		writeFile(t, dir, "scripts/validate.js", "// validator")

		body := "See package.json for available commands. Run `npm run validate` to check."
		results := CheckOrphanFiles(dir, body, Options{})

		// package.json is mentioned so it gets scanned, finding scripts/validate.js
		requireNoResultContaining(t, results, types.Warning, "scripts/validate.js")
	})

	t.Run("package.json not scanned when SKILL.md only mentions npm commands", func(t *testing.T) {
		dir := t.TempDir()
		// SKILL.md says "npm run validate" but doesn't mention package.json
		writeFile(t, dir, "package.json", `{"scripts":{"validate":"node scripts/validate.js"}}`)
		writeFile(t, dir, "scripts/validate.js", "// validator")

		body := "Run `npm run validate` to check your component."
		results := CheckOrphanFiles(dir, body, Options{})

		// package.json is not mentioned, so scripts/validate.js stays orphaned
		requireResultContaining(t, results, types.Warning, "potentially unreferenced file: scripts/validate.js")
	})

	t.Run("root file matched case-insensitively", func(t *testing.T) {
		dir := t.TempDir()
		// SKILL.md says "FORMS.md" but the file on disk is "forms.md"
		writeFile(t, dir, "forms.md", "Run scripts/fill_form.py to fill the form.")
		writeFile(t, dir, "scripts/fill_form.py", "#!/usr/bin/env python3")

		body := "For form filling, read FORMS.md and follow its instructions."
		results := CheckOrphanFiles(dir, body, Options{})

		requireNoResultContaining(t, results, types.Warning, "scripts/fill_form.py")
		requireResult(t, results, types.Pass, "all files in scripts/ are referenced")
	})

	t.Run("script referenced without extension gets specific warning", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "scripts/check_fields.py", "#!/usr/bin/env python3")

		body := "Run `python scripts/check_fields <file.pdf>` to check."
		results := CheckOrphanFiles(dir, body, Options{})

		requireResultContaining(t, results, types.Warning,
			"file scripts/check_fields.py is referenced without its extension (as scripts/check_fields in SKILL.md) — include the .py extension so agents can reliably locate the file")
		// Should NOT also emit the generic orphan warning
		requireNoResultContaining(t, results, types.Warning, "potentially unreferenced file: scripts/check_fields.py")
	})

	t.Run("extensionless match via intermediary file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "forms.md", "Run `python scripts/check_fields <file>`.")
		writeFile(t, dir, "scripts/check_fields.py", "#!/usr/bin/env python3")

		body := "For form filling, read forms.md."
		results := CheckOrphanFiles(dir, body, Options{})

		requireResultContaining(t, results, types.Warning,
			"file scripts/check_fields.py is referenced without its extension (as scripts/check_fields in forms.md)")
	})

	t.Run("__init__.py excluded from checks entirely", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "scripts/__init__.py", "")
		writeFile(t, dir, "scripts/run.py", "#!/usr/bin/env python3")

		body := "Run scripts/run.py to start."
		results := CheckOrphanFiles(dir, body, Options{})

		requireNoResultContaining(t, results, types.Warning, "__init__.py")
		requireNoResultContaining(t, results, types.Info, "__init__.py")
		requireResult(t, results, types.Pass, "all files in scripts/ are referenced")
	})

	t.Run("__init__.py not flagged even when directory is orphaned", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "scripts/__init__.py", "")
		writeFile(t, dir, "scripts/run.py", "#!/usr/bin/env python3")

		body := "No references here."
		results := CheckOrphanFiles(dir, body, Options{})

		requireNoResultContaining(t, results, types.Warning, "__init__.py")
		requireResultContaining(t, results, types.Warning, "potentially unreferenced file: scripts/run.py")
	})

	t.Run("nested __init__.py excluded from checks", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "scripts/pkg/__init__.py", "")
		writeFile(t, dir, "scripts/pkg/helpers.py", "# helpers")

		body := "No references here."
		results := CheckOrphanFiles(dir, body, Options{})

		requireNoResultContaining(t, results, types.Warning, "__init__.py")
		requireResultContaining(t, results, types.Warning, "scripts/pkg/helpers.py")
	})

	t.Run("full extension match takes priority over extensionless", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "scripts/setup.sh", "#!/bin/bash")

		// Body references with full extension — should get normal treatment, not the extension warning
		body := "Run scripts/setup.sh to configure."
		results := CheckOrphanFiles(dir, body, Options{})

		requireResult(t, results, types.Pass, "all files in scripts/ are referenced")
		requireNoResultContaining(t, results, types.Warning, "referenced without its extension")
	})

	t.Run("unreferenced root file does not get scanned", func(t *testing.T) {
		dir := t.TempDir()
		// notes.md exists at root but SKILL.md doesn't mention it
		writeFile(t, dir, "notes.md", "Run scripts/secret.sh for setup.")
		writeFile(t, dir, "scripts/secret.sh", "#!/bin/bash")

		body := "This skill has no special setup."
		results := CheckOrphanFiles(dir, body, Options{})

		// notes.md is never mentioned, so it shouldn't be scanned, and the script stays orphaned
		requireResultContaining(t, results, types.Warning, "potentially unreferenced file: scripts/secret.sh")
	})

	t.Run("Python import resolves sibling module", func(t *testing.T) {
		dir := t.TempDir()
		// SKILL.md references main.py, which imports helpers
		writeFile(t, dir, "scripts/main.py", "from helpers import merge\nmerge()")
		writeFile(t, dir, "scripts/helpers.py", "def merge(): pass")

		body := "Run scripts/main.py to start."
		results := CheckOrphanFiles(dir, body, Options{})

		requireNoResultContaining(t, results, types.Warning, "scripts/helpers.py")
		requireResult(t, results, types.Pass, "all files in scripts/ are referenced")
	})

	t.Run("Python import resolves dotted module path", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "scripts/main.py", "from helpers.merge_runs import merge\nmerge()")
		writeFile(t, dir, "scripts/helpers/__init__.py", "")
		writeFile(t, dir, "scripts/helpers/merge_runs.py", "def merge(): pass")

		body := "Run scripts/main.py to start."
		results := CheckOrphanFiles(dir, body, Options{})

		requireNoResultContaining(t, results, types.Warning, "scripts/helpers/merge_runs.py")
		requireResult(t, results, types.Pass, "all files in scripts/ are referenced")
	})

	t.Run("Python relative import resolves", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "scripts/pkg/__init__.py", "")
		writeFile(t, dir, "scripts/pkg/main.py", "from .utils import helper\nhelper()")
		writeFile(t, dir, "scripts/pkg/utils.py", "def helper(): pass")

		body := "Run scripts/pkg/main.py to start."
		results := CheckOrphanFiles(dir, body, Options{})

		requireNoResultContaining(t, results, types.Warning, "scripts/pkg/utils.py")
		requireResult(t, results, types.Pass, "all files in scripts/ are referenced")
	})

	t.Run("Python import does not match non-Python files", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "scripts/main.py", "import data_loader\ndata_loader.run()")
		writeFile(t, dir, "scripts/data_loader.sh", "#!/bin/bash")

		body := "Run scripts/main.py to start."
		results := CheckOrphanFiles(dir, body, Options{})

		// .sh file is not resolved by Python imports ("import data_loader"
		// refers to a Python module, not the shell script), and the bare
		// identifier in main.py is not treated as a same-directory reference
		// (see issue #85), so the file is reported as unreferenced.
		requireResultContaining(t, results, types.Warning,
			"potentially unreferenced file: scripts/data_loader.sh")
	})

	t.Run("__init__.py bridges package imports to sibling modules", func(t *testing.T) {
		dir := t.TempDir()
		// pack.py imports from the validators package, which is a directory
		// with __init__.py that re-exports from sibling modules.
		writeFile(t, dir, "scripts/pack.py", "from validators import BaseValidator\nBaseValidator()")
		writeFile(t, dir, "scripts/validators/__init__.py", "from .base import BaseValidator")
		writeFile(t, dir, "scripts/validators/base.py", "class BaseValidator: pass")
		writeFile(t, dir, "scripts/validators/extra.py", "class ExtraValidator: pass")

		body := "Run scripts/pack.py to package."
		results := CheckOrphanFiles(dir, body, Options{})

		// base.py should be reached via: pack.py → __init__.py → .base
		requireNoResultContaining(t, results, types.Warning, "scripts/validators/base.py")
		// extra.py is not imported by __init__.py, so it stays orphaned
		requireResultContaining(t, results, types.Warning, "potentially unreferenced file: scripts/validators/extra.py")
	})

	t.Run("multiple orphans across directories", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "references/unused1.md", "content")
		writeFile(t, dir, "scripts/unused2.sh", "content")
		writeFile(t, dir, "assets/unused3.png", "content")

		body := "No references to any files."
		results := CheckOrphanFiles(dir, body, Options{})

		requireResultContaining(t, results, types.Warning, "potentially unreferenced file: references/unused1.md")
		requireResultContaining(t, results, types.Warning, "potentially unreferenced file: scripts/unused2.sh")
		requireResultContaining(t, results, types.Warning, "potentially unreferenced file: assets/unused3.png")
	})

	t.Run("allow-dirs emits info note for existing allowed directory", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "references/guide.md", "guide content")
		writeFile(t, dir, "evals/evals.json", `{"tests": []}`)

		body := "See references/guide.md for details."
		results := CheckOrphanFiles(dir, body, Options{AllowDirs: []string{"evals"}})

		requireResultContaining(t, results, types.Info, "evals/ skipped for orphan detection (allowed via --allow-dirs)")
		requireResult(t, results, types.Pass, "all files in references/ are referenced")
	})

	t.Run("allow-dirs no info note for nonexistent allowed directory", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "references/guide.md", "guide content")

		body := "See references/guide.md for details."
		results := CheckOrphanFiles(dir, body, Options{AllowDirs: []string{"evals"}})

		requireNoResultContaining(t, results, types.Info, "evals/")
	})

	t.Run("allow-dirs skips recognized dir in info notes", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "references/guide.md", "guide content")

		body := "See references/guide.md for details."
		results := CheckOrphanFiles(dir, body, Options{AllowDirs: []string{"references"}})

		// "references" is already a recognized dir, so no info note
		requireNoResultContaining(t, results, types.Info, "references/")
		requireResult(t, results, types.Pass, "all files in references/ are referenced")
	})

	t.Run("allow-dirs with skip-orphans does not conflict", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "---\nname: test\n---\nbody")
		writeFile(t, dir, "references/guide.md", "guide content")
		writeFile(t, dir, "evals/evals.json", `{"tests": []}`)

		// When SkipOrphans is true, CheckOrphanFiles is never called (handled in Validate).
		// But if called directly, it should still work.
		results := CheckOrphanFiles(dir, "body", Options{
			AllowDirs: []string{"evals"},
		})
		requireResultContaining(t, results, types.Info, "evals/ skipped for orphan detection")
	})

	t.Run("allow-dirs with multiple allowed dirs", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "references/guide.md", "guide content")
		writeFile(t, dir, "evals/evals.json", `{"tests": []}`)
		writeFile(t, dir, "testing/test1.md", "test content")

		body := "See references/guide.md for details."
		results := CheckOrphanFiles(dir, body, Options{AllowDirs: []string{"evals", "testing"}})

		requireResultContaining(t, results, types.Info, "evals/ skipped for orphan detection")
		requireResultContaining(t, results, types.Info, "testing/ skipped for orphan detection")
		requireResult(t, results, types.Pass, "all files in references/ are referenced")
	})
}

func TestContainsReference(t *testing.T) {
	t.Run("forward-slash path matches markdown text", func(t *testing.T) {
		// Inventory paths are normalized to forward slashes; markdown uses forward slashes.
		if !containsReference("See references/other.md for details.", "", "references/other.md") {
			t.Error("expected forward-slash inventory path to match forward-slash markdown reference")
		}
	})

	t.Run("relative path from subdirectory matches", func(t *testing.T) {
		// Source is in references/, relPath is references/images/diagram.png,
		// so the relative path is images/diagram.png.
		text := "See ![diagram](images/diagram.png)."
		if !containsReference(text, "references", "references/images/diagram.png") {
			t.Error("expected relative path from subdirectory to match")
		}
	})

	t.Run("does not match inside a longer filename", func(t *testing.T) {
		if containsReference("Configure eslint.yml before running.", "references", "references/lint.yml") {
			t.Error("lint.yml should not match inside eslint.yml")
		}
		if containsReference("See references/linting.md for details.", "", "references/lint.md") {
			t.Error("references/lint.md should not match inside references/linting.md")
		}
	})

	t.Run("same-directory mention with extension still matches", func(t *testing.T) {
		if !containsReference("The workflow lives in lint.yml in this directory.", "references", "references/lint.yml") {
			t.Error("expected bare-filename mention with extension to match")
		}
	})
}

func TestContainsReferenceWithoutExtension(t *testing.T) {
	// Issue #85: a same-directory sibling reduces the extensionless form to a
	// bare word, which must not match ordinary prose.
	t.Run("bare word in prose does not match", func(t *testing.T) {
		text := "Run the lint step before committing. Linting catches most problems."
		if containsReferenceWithoutExtension(text, "references", "references/lint.yml") {
			t.Error("prose word 'lint' should not count as a reference to references/lint.yml")
		}
	})

	t.Run("extensionless path with separator matches", func(t *testing.T) {
		text := "Run scripts/check_fillable_fields to inspect the form."
		if !containsReferenceWithoutExtension(text, "", "scripts/check_fillable_fields.py") {
			t.Error("expected extensionless path with directory to match")
		}
	})

	t.Run("extensionless path respects word boundaries", func(t *testing.T) {
		if containsReferenceWithoutExtension("See references/linting for details.", "", "references/lint.yml") {
			t.Error("references/lint should not match inside references/linting")
		}
	})

	t.Run("relative extensionless path with separator matches", func(t *testing.T) {
		text := "Run images/render to regenerate."
		if !containsReferenceWithoutExtension(text, "references", "references/images/render.py") {
			t.Error("expected sourceDir-relative extensionless path to match")
		}
	})
}

func TestContainsPathToken(t *testing.T) {
	tests := []struct {
		text, token string
		want        bool
	}{
		{"see references/lint.yml here", "references/lint.yml", true},
		{"references/lint.yml", "references/lint.yml", true},
		{"(references/lint.yml)", "references/lint.yml", true},
		{"./references/lint.yml", "references/lint.yml", true},
		{"my-references/lint.yml", "references/lint.yml", false},
		{"references/lint.yml2", "references/lint.yml", false},
		{"eslint.yml", "lint.yml", false},
		{"lint.yml.bak then lint.yml", "lint.yml", true},
		{"", "lint.yml", false},
	}
	for _, tt := range tests {
		if got := containsPathToken(tt.text, tt.token); got != tt.want {
			t.Errorf("containsPathToken(%q, %q) = %v, want %v", tt.text, tt.token, got, tt.want)
		}
	}
}

func TestFilesInDir(t *testing.T) {
	t.Run("forward-slash inventory matches directory prefix", func(t *testing.T) {
		inventory := []string{
			"references/guide.md",
			"references/other.md",
			"scripts/setup.sh",
		}
		got := filesInDir(inventory, "references")
		if len(got) != 2 {
			t.Errorf("expected 2 files in references/, got %d: %v", len(got), got)
		}
	})
}

func TestPythonImportReaches(t *testing.T) {
	t.Run("dotted import resolves with forward slashes", func(t *testing.T) {
		// "from helpers.merge_runs import merge" in scripts/main.py should
		// resolve to scripts/helpers/merge_runs.py (forward slashes).
		text := "from helpers.merge_runs import merge"
		if !pythonImportReaches(text, "scripts/main.py", "scripts/helpers/merge_runs.py") {
			t.Error("expected dotted Python import to resolve to forward-slash path")
		}
	})

	t.Run("relative import resolves with forward slashes", func(t *testing.T) {
		text := "from .utils import helper"
		if !pythonImportReaches(text, "scripts/pkg/main.py", "scripts/pkg/utils.py") {
			t.Error("expected relative Python import to resolve to forward-slash path")
		}
	})
}

func TestCheckFlatOrphanFiles(t *testing.T) {
	t.Run("all root files referenced", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "guide.md", "guide content")
		writeFile(t, dir, "notes.txt", "notes")
		body := "See guide.md for details and notes.txt for notes."
		results := CheckFlatOrphanFiles(dir, body)
		requireResult(t, results, types.Pass, "all root-level files are referenced from SKILL.md")
		requireNoLevel(t, results, types.Warning)
	})

	t.Run("unreferenced root file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "guide.md", "guide content")
		writeFile(t, dir, "orphan.txt", "orphan")
		body := "See guide.md for details."
		results := CheckFlatOrphanFiles(dir, body)
		requireResultContaining(t, results, types.Warning, "potentially unreferenced file: orphan.txt")
		requireNoResultContaining(t, results, types.Warning, "guide.md")
	})

	t.Run("no root files besides SKILL.md", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "SKILL.md", "content")
		results := CheckFlatOrphanFiles(dir, "body")
		if len(results) != 0 {
			t.Errorf("expected no results, got %d", len(results))
		}
	})

	t.Run("extensionless reference still matches", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "helper.py", "def help(): pass")
		body := "Use helper to do things."
		results := CheckFlatOrphanFiles(dir, body)
		requireNoLevel(t, results, types.Warning)
	})

	t.Run("hidden files are skipped", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, ".hidden", "secret")
		writeFile(t, dir, "visible.md", "content")
		body := "See visible.md."
		results := CheckFlatOrphanFiles(dir, body)
		requireNoResultContaining(t, results, types.Warning, ".hidden")
	})

	t.Run("directories are skipped", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "scripts/run.sh", "#!/bin/bash")
		writeFile(t, dir, "guide.md", "guide")
		body := "See guide.md."
		results := CheckFlatOrphanFiles(dir, body)
		// Should only report on root files, not dir contents
		requireNoResultContaining(t, results, types.Warning, "scripts")
	})
}
