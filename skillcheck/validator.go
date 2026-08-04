// Package skillcheck provides skill detection and reference analysis
// operations. Type definitions (Level, Result, Report, etc.) live in
// the types package.
package skillcheck

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agent-ecosystem/skill-validator/contamination"
	"github.com/agent-ecosystem/skill-validator/content"
	"github.com/agent-ecosystem/skill-validator/types"
	"github.com/agent-ecosystem/skill-validator/util"
)

// DetectSkills determines whether dir is a single skill, a multi-skill
// parent, or contains no skills. It follows symlinks when checking
// subdirectories.
func DetectSkills(dir string) (types.SkillMode, []string) {
	// If the directory itself contains SKILL.md, it's a single skill.
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
		return types.SingleSkill, []string{dir}
	}

	// Scan immediate subdirectories for SKILL.md.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return types.NoSkill, nil
	}

	var skillDirs []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		subdir := filepath.Join(dir, name)
		// Use os.Stat (not entry.IsDir()) to follow symlinks.
		info, err := os.Stat(subdir)
		if err != nil || !info.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(subdir, "SKILL.md")); err == nil {
			skillDirs = append(skillDirs, subdir)
		}
	}

	if len(skillDirs) > 0 {
		return types.MultiSkill, skillDirs
	}
	return types.NoSkill, nil
}

// ReadSkillRaw reads the raw SKILL.md content from a directory without parsing
// frontmatter. This is used as a fallback for content/contamination analysis when
// frontmatter parsing fails.
func ReadSkillRaw(dir string) string {
	data, err := util.SafeReadFile(dir, filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// ReadReferencesMarkdownFiles reads all .md files from <dir>/references/ and returns
// a map from filename to content. Returns nil if no references dir or no .md files
// are found.
func ReadReferencesMarkdownFiles(dir string) map[string]string {
	refsDir := filepath.Join(dir, "references")
	entries, err := os.ReadDir(refsDir)
	if err != nil {
		return nil
	}

	files := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		data, err := util.SafeReadFile(dir, filepath.Join(refsDir, entry.Name()))
		if err != nil {
			continue
		}
		files[entry.Name()] = string(data)
	}

	if len(files) == 0 {
		return nil
	}
	return files
}

// AnalyzeReferences runs content and contamination analysis on reference markdown
// files. It populates the aggregate ReferencesContentReport, ReferencesContaminationReport,
// and per-file ReferenceReports on the given report.
func AnalyzeReferences(dir string, rpt *types.Report) {
	files := ReadReferencesMarkdownFiles(dir)
	if files == nil {
		return
	}

	// Sort filenames for deterministic ordering
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	// Per-file analysis
	var parts []string
	for _, name := range names {
		fileContent := files[name]
		parts = append(parts, fileContent)

		fr := types.ReferenceFileReport{File: name}
		fr.ContentReport = content.Analyze(fileContent)
		skillName := util.SkillNameFromDir(dir)
		fr.ContaminationReport = contamination.Analyze(skillName, fileContent, fr.ContentReport.CodeLanguages)
		rpt.ReferenceReports = append(rpt.ReferenceReports, fr)
	}

	// Aggregate analysis on concatenated content
	concatenated := strings.Join(parts, "\n")
	rpt.ReferencesContentReport = content.Analyze(concatenated)
	skillName := util.SkillNameFromDir(dir)
	rpt.ReferencesContaminationReport = contamination.Analyze(skillName, concatenated, rpt.ReferencesContentReport.CodeLanguages)
}
