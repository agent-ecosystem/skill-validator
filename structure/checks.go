package structure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agent-ecosystem/skill-validator/types"
	"github.com/agent-ecosystem/skill-validator/util"
)

// recognizedDirs lists the directory names defined by the skill spec.
var recognizedDirs = map[string]bool{
	"scripts":    true,
	"references": true,
	"assets":     true,
}

// Files commonly found in repos but not intended for agent consumption.
// Per Anthropic best practices: "A skill should only contain essential files
// that directly support its functionality."
// See: github.com/anthropics/skills → skill-creator
var knownExtraneousFiles = map[string]string{
	"readme.md":             "README.md",
	"readme":                "README",
	"changelog.md":          "CHANGELOG.md",
	"changelog":             "CHANGELOG",
	"license":               "LICENSE",
	"license.md":            "LICENSE.md",
	"license.txt":           "LICENSE.txt",
	"contributing.md":       "CONTRIBUTING.md",
	"code_of_conduct.md":    "CODE_OF_CONDUCT.md",
	"installation_guide.md": "INSTALLATION_GUIDE.md",
	"quick_reference.md":    "QUICK_REFERENCE.md",
	"makefile":              "Makefile",
	".gitignore":            ".gitignore",
	"owners.yaml":           "OWNERS.yaml",
	"owners":                "OWNERS",
}

// CheckStructure validates the directory layout of a skill package. It checks
// for the required SKILL.md file, flags unrecognized directories and extraneous
// root files, and warns about deep nesting in recognized directories.
// Directories listed in opts.AllowDirs are accepted without warning and are
// exempt from deep-nesting checks. Paths listed in opts.AllowNestedPaths are
// exempt only from deep-nesting warnings within those subtrees.
func CheckStructure(dir string, opts Options) []types.Result {
	ctx := types.ResultContext{Category: "Structure"}
	var results []types.Result

	// Build a set of user-allowed directories.
	allowedDirs := make(map[string]bool, len(opts.AllowDirs))
	for _, d := range opts.AllowDirs {
		allowedDirs[d] = true
	}

	allowedNestedPaths := make([]string, 0, len(opts.AllowNestedPaths))
	for _, p := range opts.AllowNestedPaths {
		normalized, err := normalizeRelativePath(p)
		if err == nil {
			allowedNestedPaths = append(allowedNestedPaths, normalized)
		}
	}

	// Check SKILL.md exists
	skillPath := filepath.Join(dir, "SKILL.md")
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		results = append(results, ctx.ErrorFile("SKILL.md", "SKILL.md not found"))
		return results
	}
	results = append(results, ctx.PassFile("SKILL.md", "SKILL.md found"))

	// Check directories
	entries, err := os.ReadDir(dir)
	if err != nil {
		results = append(results, ctx.Errorf("reading directory: %v", err))
		return results
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue // skip hidden files/dirs
		}
		if !entry.IsDir() {
			if name != "SKILL.md" && !opts.AllowFlatLayouts {
				results = append(results, extraneousFileResult(ctx, name))
			}
			continue
		}
		if !recognizedDirs[name] && !allowedDirs[name] {
			msg := fmt.Sprintf("unknown directory: %s/", name)
			if subEntries, err := os.ReadDir(filepath.Join(dir, name)); err == nil {
				fileCount := 0
				for _, se := range subEntries {
					if !strings.HasPrefix(se.Name(), ".") {
						fileCount++
					}
				}
				if fileCount > 0 {
					hint := unknownDirHint(dir)
					msg = fmt.Sprintf(
						"unknown directory: %s/ (contains %d file%s) — agents using the standard skill structure won't discover these files%s",
						name, fileCount, util.PluralS(fileCount), hint,
					)
				}
			}
			results = append(results, ctx.Warn(msg))
		}
	}

	// Check for deep nesting in recognized directories only.
	// Allowed directories are exempt because the validator cannot know
	// their expected internal structure.
	for dirName := range recognizedDirs {
		subdir := filepath.Join(dir, dirName)
		if _, err := os.Stat(subdir); os.IsNotExist(err) {
			continue
		}
		err := checkNesting(ctx, subdir, dirName, allowedNestedPaths)
		if err != nil {
			results = append(results, err...)
		}
	}

	return results
}

func extraneousFileResult(ctx types.ResultContext, name string) types.Result {
	lower := strings.ToLower(name)
	if lower == "agents.md" {
		return ctx.WarnFile(name, fmt.Sprintf(
			"%s is for repo-level agent configuration, not skill content — "+
				"move it outside the skill directory (e.g. to the repository root) "+
				"where agents discover it automatically",
			name,
		))
	}
	if _, known := knownExtraneousFiles[lower]; known {
		return ctx.WarnFile(name, fmt.Sprintf(
			"%s is not needed in a skill — agents may load it into their context window, "+
				"taking space from your actual task (Anthropic best practices: skills should only "+
				"contain files that directly support agent functionality)",
			name,
		))
	}
	return ctx.WarnFile(name, fmt.Sprintf(
		"unexpected file at root: %s — if agents need this file, move it into "+
			"references/ or assets/ as appropriate; otherwise remove it to avoid "+
			"unnecessary context window usage",
		name,
	))
}

func unknownDirHint(dir string) string {
	var candidates []string
	if _, err := os.Stat(filepath.Join(dir, "references")); os.IsNotExist(err) {
		candidates = append(candidates, "references/")
	}
	if _, err := os.Stat(filepath.Join(dir, "assets")); os.IsNotExist(err) {
		candidates = append(candidates, "assets/")
	}
	if len(candidates) == 0 {
		return ""
	}
	return fmt.Sprintf("; should this be %s?", strings.Join(candidates, " or "))
}

func checkNesting(ctx types.ResultContext, dir, prefix string, allowedPaths []string) []types.Result {
	var results []types.Result
	entries, err := os.ReadDir(dir)
	if err != nil {
		return results
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if entry.IsDir() {
			nestedPath := filepath.ToSlash(filepath.Join(prefix, entry.Name()))
			allowed := false
			for _, allowedPath := range allowedPaths {
				if pathInSubtree(nestedPath, allowedPath) {
					allowed = true
					break
				}
			}
			if allowed {
				results = append(results, ctx.Infof("deep nesting allowed: %s/", nestedPath))
				continue
			}
			results = append(results, ctx.Warnf("deep nesting detected: %s/", nestedPath))
		}
	}
	return results
}
