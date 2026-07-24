// Package structure validates the directory layout, frontmatter, token counts,
// markdown syntax, internal links, and orphan files of a skill package. It is
// the main validation entry point used by the CLI.
package structure

import (
	"github.com/agent-ecosystem/skill-validator/skill"
	"github.com/agent-ecosystem/skill-validator/types"
	"github.com/agent-ecosystem/skill-validator/util"
)

// Options configures which checks Validate runs.
type Options struct {
	SkipOrphans           bool
	AllowExtraFrontmatter bool
	AllowFlatLayouts      bool
	AllowDirs             []string
	AllowNestedPaths      []string
}

// ValidateMulti validates each directory and returns an aggregated report.
func ValidateMulti(dirs []string, opts Options) *types.MultiReport {
	mr := &types.MultiReport{}
	for _, dir := range dirs {
		r := Validate(dir, opts)
		mr.Skills = append(mr.Skills, r)
		mr.Errors += r.Errors
		mr.Warnings += r.Warnings
	}
	return mr
}

// Validate runs all checks against the skill in the given directory.
func Validate(dir string, opts Options) *types.Report {
	report := &types.Report{SkillDir: dir}

	// Structure checks
	structResults := CheckStructure(dir, opts)
	report.Results = append(report.Results, structResults...)

	// Check if SKILL.md was found; if not, skip further checks
	hasSkillMD := false
	for _, r := range structResults {
		if r.Level == types.Pass && r.Message == "SKILL.md found" {
			hasSkillMD = true
			break
		}
	}
	if !hasSkillMD {
		report.Tally()
		return report
	}

	// Parse skill
	s, err := skill.Load(dir)
	if err != nil {
		report.Results = append(report.Results,
			types.ResultContext{Category: "Frontmatter", File: "SKILL.md"}.Error(err.Error()))
		report.Tally()
		return report
	}

	// Frontmatter checks
	report.Results = append(report.Results, CheckFrontmatter(s, opts)...)

	// Token checks
	tokenResults, tokenCounts, otherCounts := CheckTokens(dir, s.Body, opts)
	report.Results = append(report.Results, tokenResults...)
	report.TokenCounts = tokenCounts
	report.OtherTokenCounts = otherCounts

	// Holistic structure check: is this actually a skill?
	report.Results = append(report.Results, checkSkillRatio(report.TokenCounts, report.OtherTokenCounts)...)

	// Markdown structure checks (unclosed code fences)
	report.Results = append(report.Results, CheckMarkdown(dir, s.Body)...)

	// Internal link checks (broken relative links are a structural issue)
	report.Results = append(report.Results, CheckInternalLinks(dir, s.Body)...)

	// Orphan file checks (files in recognized dirs that are never referenced)
	if !opts.SkipOrphans {
		report.Results = append(report.Results, CheckOrphanFiles(dir, s.Body, opts)...)
		if opts.AllowFlatLayouts {
			report.Results = append(report.Results, CheckFlatOrphanFiles(dir, s.Body)...)
		}
	}

	report.Tally()
	return report
}

func checkSkillRatio(standard, other []types.TokenCount) []types.Result {
	ctx := types.ResultContext{Category: "Overall"}
	standardTotal := 0
	for _, tc := range standard {
		standardTotal += tc.Tokens
	}
	otherTotal := 0
	for _, tc := range other {
		otherTotal += tc.Tokens
	}

	if otherTotal > 25_000 && standardTotal > 0 && otherTotal > standardTotal*10 {
		return []types.Result{ctx.Errorf(
			"this content doesn't appear to be structured as a skill — "+
				"there are %s tokens of non-standard content but only %s tokens in the "+
				"standard skill structure (SKILL.md + references). This ratio suggests a "+
				"build pipeline issue or content that belongs in a different format, not a skill. "+
				"Per the spec, a skill should contain a focused SKILL.md with optional references, "+
				"scripts, and assets.",
			util.FormatNumber(otherTotal), util.FormatNumber(standardTotal),
		)}
	}

	return nil
}
