package structure

import (
	"path/filepath"
	"testing"

	"github.com/agent-ecosystem/skill-validator/skillcheck"
	"github.com/agent-ecosystem/skill-validator/types"
)

func TestValidate(t *testing.T) {
	t.Run("valid skill", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: "+dirName(dir)+"\ndescription: A valid skill\n---\n# Body\n")
		report := Validate(dir, Options{})
		if report.Errors != 0 {
			t.Errorf("expected 0 errors, got %d", report.Errors)
			for _, r := range report.Results {
				if r.Level == types.Error {
					t.Logf("  error: %s: %s", r.Category, r.Message)
				}
			}
		}
	})

	t.Run("missing SKILL.md stops early", func(t *testing.T) {
		dir := t.TempDir()
		report := Validate(dir, Options{})
		if report.Errors != 1 {
			t.Errorf("expected 1 error, got %d", report.Errors)
		}
		requireResult(t, report.Results, types.Error, "SKILL.md not found")
		// Should not have any frontmatter/link/token results
		for _, r := range report.Results {
			if r.Category != "Structure" {
				t.Errorf("unexpected category %q when SKILL.md missing", r.Category)
			}
		}
	})

	t.Run("multiple validation errors", func(t *testing.T) {
		dir := t.TempDir()
		// Invalid name, missing description, broken link
		writeSkill(t, dir, "---\nname: BAD\ndescription: \"\"\n---\n[broken](references/nope.md)\n")
		report := Validate(dir, Options{})
		if report.Errors < 3 {
			t.Errorf("expected at least 3 errors, got %d", report.Errors)
			for _, r := range report.Results {
				if r.Level == types.Error {
					t.Logf("  error: %s: %s", r.Category, r.Message)
				}
			}
		}
	})

	t.Run("tally counts errors and warnings", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: "+dirName(dir)+"\ndescription: desc\ncustom: field\n---\n# Body\n")
		writeFile(t, dir, "extras/file.txt", "content")
		report := Validate(dir, Options{})
		if report.Warnings < 1 {
			t.Errorf("expected at least 1 warning, got %d", report.Warnings)
		}
	})

	t.Run("token counts populated", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: "+dirName(dir)+"\ndescription: desc\n---\n# Body content\n")
		writeFile(t, dir, "references/ref.md", "Reference text.")
		report := Validate(dir, Options{})
		if len(report.TokenCounts) != 2 {
			t.Errorf("expected 2 token counts, got %d", len(report.TokenCounts))
		}
	})

	t.Run("other token counts populated", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: "+dirName(dir)+"\ndescription: desc\n---\n# Body content\n")
		writeFile(t, dir, "AGENTS.md", "Some agent content here.")
		writeFile(t, dir, "rules/rule1.md", "Rule one.")
		report := Validate(dir, Options{})
		if len(report.OtherTokenCounts) != 2 {
			t.Errorf("expected 2 other token counts, got %d", len(report.OtherTokenCounts))
			for _, c := range report.OtherTokenCounts {
				t.Logf("  %s: %d tokens", c.File, c.Tokens)
			}
		}
	})

	t.Run("no other token counts for standard structure", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: "+dirName(dir)+"\ndescription: desc\n---\n# Body content\n")
		writeFile(t, dir, "references/ref.md", "Reference text.")
		report := Validate(dir, Options{})
		if len(report.OtherTokenCounts) != 0 {
			t.Errorf("expected 0 other token counts, got %d", len(report.OtherTokenCounts))
		}
	})

	t.Run("not structured as a skill error", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: "+dirName(dir)+"\ndescription: desc\n---\n# Body\n")
		// Add a massive amount of non-standard content
		writeFile(t, dir, "AGENTS.md", generateContent(30_000))
		report := Validate(dir, Options{})
		requireResultContaining(t, report.Results, types.Error, "doesn't appear to be structured as a skill")
		requireResultContaining(t, report.Results, types.Error, "build pipeline issue")
	})

	t.Run("no skill ratio error when other content is small", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: "+dirName(dir)+"\ndescription: desc\n---\n# Body\n")
		writeFile(t, dir, "extra.md", "A small extra file.")
		report := Validate(dir, Options{})
		requireNoResultContaining(t, report.Results, types.Error, "doesn't appear to be structured as a skill")
	})

	t.Run("flat layout no warnings for root files", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: "+dirName(dir)+"\ndescription: A flat skill\n---\n# Body\nSee guide.md for details.\n")
		writeFile(t, dir, "guide.md", "Guide content.")
		report := Validate(dir, Options{AllowFlatLayouts: true})
		if report.Errors != 0 {
			t.Errorf("expected 0 errors, got %d", report.Errors)
			for _, r := range report.Results {
				if r.Level == types.Error {
					t.Logf("  error: %s: %s", r.Category, r.Message)
				}
			}
		}
		if report.Warnings != 0 {
			t.Errorf("expected 0 warnings, got %d", report.Warnings)
			for _, r := range report.Results {
				if r.Level == types.Warning {
					t.Logf("  warning: %s: %s", r.Category, r.Message)
				}
			}
		}
		// Root file should be in standard counts, not other
		if len(report.OtherTokenCounts) != 0 {
			t.Errorf("expected 0 other token counts, got %d", len(report.OtherTokenCounts))
		}
	})

	t.Run("flat layout warns on unreferenced root files", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: "+dirName(dir)+"\ndescription: A flat skill\n---\n# Body\n")
		writeFile(t, dir, "orphan.md", "Nobody references me.")
		report := Validate(dir, Options{AllowFlatLayouts: true})
		requireResultContaining(t, report.Results, types.Warning, "potentially unreferenced file: orphan.md")
	})

	t.Run("flat layout large content not flagged as non-skill", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: "+dirName(dir)+"\ndescription: desc\n---\n# Body\nSee big-ref.md.\n")
		writeFile(t, dir, "big-ref.md", generateContent(30_000))
		report := Validate(dir, Options{AllowFlatLayouts: true})
		requireNoResultContaining(t, report.Results, types.Error, "doesn't appear to be structured as a skill")
	})

	t.Run("unparseable frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\n: invalid: yaml: [broken\n---\nBody\n")
		report := Validate(dir, Options{})
		if report.Errors != 1 {
			t.Errorf("expected 1 error, got %d", report.Errors)
		}
		requireResultContaining(t, report.Results, types.Error, "parsing frontmatter YAML")
	})

	t.Run("allow-nested-paths changes only nesting results", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: "+dirName(dir)+"\ndescription: desc\n---\n# Body\n")
		writeFile(t, dir, "assets/components/button/include.md", "unreferenced asset")

		report := Validate(dir, Options{AllowNestedPaths: []string{"assets/components"}})

		requireNoResultContaining(t, report.Results, types.Warning, "deep nesting detected: assets/components/")
		requireResultContaining(t, report.Results, types.Info, "deep nesting allowed: assets/components/")
		requireResultContaining(t, report.Results, types.Warning, "potentially unreferenced file: assets/components/button/include.md")

		hasAssetTokenCount := false
		for _, count := range report.TokenCounts {
			if count.File == "assets/components/button/include.md" {
				hasAssetTokenCount = true
				break
			}
		}
		if !hasAssetTokenCount {
			t.Error("expected allowed nested path to remain in token accounting")
		}
	})

	t.Run("allow-dirs suppresses unknown dir warning in full validation", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: "+dirName(dir)+"\ndescription: A valid skill\n---\n# Body\n")
		writeFile(t, dir, "evals/evals.json", `{"tests": []}`)
		report := Validate(dir, Options{AllowDirs: []string{"evals"}})
		if report.Errors != 0 {
			t.Errorf("expected 0 errors, got %d", report.Errors)
		}
		requireNoResultContaining(t, report.Results, types.Warning, "evals/")
	})

	t.Run("allow-dirs with skip-orphans", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: "+dirName(dir)+"\ndescription: A valid skill\n---\n# Body\n")
		writeFile(t, dir, "evals/evals.json", `{"tests": []}`)
		writeFile(t, dir, "references/unused.md", "unused content")
		report := Validate(dir, Options{AllowDirs: []string{"evals"}, SkipOrphans: true})
		if report.Errors != 0 {
			t.Errorf("expected 0 errors, got %d", report.Errors)
		}
		// No unknown dir warning for evals
		requireNoResultContaining(t, report.Results, types.Warning, "evals/")
		// Orphan check skipped entirely, so no orphan warning for unused.md
		requireNoResultContaining(t, report.Results, types.Warning, "unreferenced")
		// No info note about allowed dirs since orphan detection was skipped
		requireNoResultContaining(t, report.Results, types.Info, "skipped for orphan detection")
	})

	t.Run("allow-dirs with allow-extra-frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: "+dirName(dir)+"\ndescription: desc\ncustom_field: value\n---\n# Body\n")
		writeFile(t, dir, "evals/evals.json", `{"tests": []}`)
		report := Validate(dir, Options{
			AllowDirs:             []string{"evals"},
			AllowExtraFrontmatter: true,
		})
		if report.Errors != 0 {
			t.Errorf("expected 0 errors, got %d", report.Errors)
		}
		requireNoResultContaining(t, report.Results, types.Warning, "evals/")
		requireNoResultContaining(t, report.Results, types.Warning, "custom_field")
	})

	t.Run("allow-dirs with allow-flat-layouts full integration", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: "+dirName(dir)+"\ndescription: desc\n---\n# Body\nSee guide.md.\n")
		writeFile(t, dir, "guide.md", "Guide content.")
		writeFile(t, dir, "evals/evals.json", `{"tests": []}`)
		report := Validate(dir, Options{
			AllowDirs:        []string{"evals"},
			AllowFlatLayouts: true,
		})
		if report.Errors != 0 {
			t.Errorf("expected 0 errors, got %d", report.Errors)
		}
		if report.Warnings != 0 {
			t.Errorf("expected 0 warnings, got %d", report.Warnings)
			for _, r := range report.Results {
				if r.Level == types.Warning {
					t.Logf("  warning: %s: %s", r.Category, r.Message)
				}
			}
		}
	})

	t.Run("allow-dirs emits info note during orphan detection", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: "+dirName(dir)+"\ndescription: desc\n---\n# Body\nSee references/guide.md.\n")
		writeFile(t, dir, "references/guide.md", "guide content")
		writeFile(t, dir, "evals/evals.json", `{"tests": []}`)
		report := Validate(dir, Options{AllowDirs: []string{"evals"}})
		requireResultContaining(t, report.Results, types.Info, "evals/ skipped for orphan detection")
	})

	t.Run("allow-dirs with all flags combined", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: "+dirName(dir)+"\ndescription: desc\ncustom: val\n---\n# Body\nSee helper.md.\n")
		writeFile(t, dir, "helper.md", "Helper content.")
		writeFile(t, dir, "evals/evals.json", `{"tests": []}`)
		writeFile(t, dir, "testing/test1.md", "test content")
		report := Validate(dir, Options{
			AllowDirs:             []string{"evals", "testing"},
			AllowExtraFrontmatter: true,
			AllowFlatLayouts:      true,
			SkipOrphans:           true,
		})
		if report.Errors != 0 {
			t.Errorf("expected 0 errors, got %d", report.Errors)
			for _, r := range report.Results {
				if r.Level == types.Error {
					t.Logf("  error: %s: %s", r.Category, r.Message)
				}
			}
		}
		if report.Warnings != 0 {
			t.Errorf("expected 0 warnings, got %d", report.Warnings)
			for _, r := range report.Results {
				if r.Level == types.Warning {
					t.Logf("  warning: %s: %s", r.Category, r.Message)
				}
			}
		}
	})
}

func TestValidateMulti(t *testing.T) {
	dir := t.TempDir()
	// Create two skills: one valid, one invalid
	goodDir := filepath.Join(dir, "good")
	writeSkill(t, goodDir, "---\nname: good\ndescription: A good skill\n---\n# Body\n")
	badDir := filepath.Join(dir, "bad")
	writeSkill(t, badDir, "---\nname: BAD\ndescription: \"\"\n---\n# Body\n")

	mr := ValidateMulti([]string{goodDir, badDir}, Options{})

	if len(mr.Skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(mr.Skills))
	}
	if mr.Skills[0].Errors != 0 {
		t.Errorf("expected good skill to have 0 errors, got %d", mr.Skills[0].Errors)
	}
	if mr.Skills[1].Errors == 0 {
		t.Errorf("expected bad skill to have errors")
	}
	if mr.Errors != mr.Skills[0].Errors+mr.Skills[1].Errors {
		t.Errorf("expected aggregated errors %d, got %d", mr.Skills[0].Errors+mr.Skills[1].Errors, mr.Errors)
	}
	if mr.Warnings != mr.Skills[0].Warnings+mr.Skills[1].Warnings {
		t.Errorf("expected aggregated warnings %d, got %d", mr.Skills[0].Warnings+mr.Skills[1].Warnings, mr.Warnings)
	}
}

func TestValidate_MultiSkillFixture(t *testing.T) {
	// Integration test using testdata/multi-skill
	fixtureDir := "../testdata/multi-skill"
	mode, dirs := skillcheck.DetectSkills(fixtureDir)
	if mode != types.MultiSkill {
		t.Fatalf("expected MultiSkill, got %d", mode)
	}
	if len(dirs) != 3 {
		t.Fatalf("expected 3 skill dirs, got %d: %v", len(dirs), dirs)
	}

	mr := ValidateMulti(dirs, Options{})
	if len(mr.Skills) != 3 {
		t.Fatalf("expected 3 skills, got %d", len(mr.Skills))
	}

	// skill-alpha and skill-beta should pass
	for _, r := range mr.Skills {
		base := filepath.Base(r.SkillDir)
		if base == "skill-alpha" || base == "skill-beta" {
			if r.Errors != 0 {
				t.Errorf("%s: expected 0 errors, got %d", base, r.Errors)
				for _, res := range r.Results {
					if res.Level == types.Error {
						t.Logf("  %s: %s", res.Category, res.Message)
					}
				}
			}
		}
		// skill-gamma should have errors
		if base == "skill-gamma" {
			if r.Errors == 0 {
				t.Errorf("skill-gamma: expected errors, got 0")
			}
		}
	}

	if mr.Errors == 0 {
		t.Error("expected aggregated errors > 0")
	}
}
