// Package orchestrate provides the core validation and analysis orchestration
// for skill directories. It coordinates calls to structure, content,
// contamination, and link checking packages, returning unified reports.
//
// This package is intended for library consumers who want to run skill
// validation without the CLI layer.
package orchestrate

import (
	"context"

	"github.com/agent-ecosystem/skill-validator/contamination"
	"github.com/agent-ecosystem/skill-validator/content"
	"github.com/agent-ecosystem/skill-validator/links"
	"github.com/agent-ecosystem/skill-validator/skill"
	"github.com/agent-ecosystem/skill-validator/skillcheck"
	"github.com/agent-ecosystem/skill-validator/structure"
	"github.com/agent-ecosystem/skill-validator/types"
	"github.com/agent-ecosystem/skill-validator/util"
)

// CheckGroup identifies a category of checks that can be enabled or disabled.
type CheckGroup string

const (
	// GroupStructure enables directory layout, frontmatter, and token checks.
	GroupStructure CheckGroup = "structure"
	// GroupLinks enables external HTTP/HTTPS link validation.
	GroupLinks CheckGroup = "links"
	// GroupContent enables content quality analysis.
	GroupContent CheckGroup = "content"
	// GroupContamination enables cross-language contamination analysis.
	GroupContamination CheckGroup = "contamination"
)

// AllGroups returns a map with all check groups enabled.
func AllGroups() map[CheckGroup]bool {
	return map[CheckGroup]bool{
		GroupStructure:     true,
		GroupLinks:         true,
		GroupContent:       true,
		GroupContamination: true,
	}
}

// Options controls which checks RunAllChecks performs.
type Options struct {
	Enabled    map[CheckGroup]bool
	StructOpts structure.Options
}

// RunAllChecks runs all enabled check groups against a single skill directory
// and returns a unified report. The context is used for cancellation of
// network operations (e.g. link checking).
func RunAllChecks(ctx context.Context, dir string, opts Options) *types.Report {
	rpt := &types.Report{SkillDir: dir}

	// Structure validation (spec compliance, tokens, code fences)
	if opts.Enabled[GroupStructure] {
		vr := structure.Validate(dir, opts.StructOpts)
		rpt.Results = append(rpt.Results, vr.Results...)
		rpt.TokenCounts = vr.TokenCounts
		rpt.OtherTokenCounts = vr.OtherTokenCounts
	}

	// Load skill for links/content/contamination checks
	needsSkill := opts.Enabled[GroupLinks] || opts.Enabled[GroupContent] || opts.Enabled[GroupContamination]
	var rawContent, body string
	var skillLoaded bool
	if needsSkill {
		s, err := skill.Load(dir)
		if err != nil {
			if !opts.Enabled[GroupStructure] {
				// Only add the error if structure didn't already catch it
				rpt.Results = append(rpt.Results,
					types.ResultContext{Category: "Skill"}.Error(err.Error()))
			}
			// Fall back to reading raw SKILL.md for content/contamination analysis
			rawContent = skillcheck.ReadSkillRaw(dir)
		} else {
			rawContent = s.RawContent
			body = s.Body
			skillLoaded = true
		}

		// Link checks require a fully parsed skill
		if skillLoaded && opts.Enabled[GroupLinks] {
			rpt.Results = append(rpt.Results, links.CheckLinks(ctx, dir, body)...)
		}

		// Content analysis works on raw content (no frontmatter parsing needed)
		if opts.Enabled[GroupContent] && rawContent != "" {
			cr := content.Analyze(rawContent)
			rpt.ContentReport = cr
		}

		// Contamination analysis works on raw content
		if opts.Enabled[GroupContamination] && rawContent != "" {
			var codeLanguages []string
			if rpt.ContentReport != nil {
				codeLanguages = rpt.ContentReport.CodeLanguages
			} else {
				cr := content.Analyze(rawContent)
				codeLanguages = cr.CodeLanguages
			}
			skillName := util.SkillNameFromDir(dir)
			rpt.ContaminationReport = contamination.Analyze(skillName, rawContent, codeLanguages)
		}

		// Reference file analysis (both content and contamination)
		if opts.Enabled[GroupContent] || opts.Enabled[GroupContamination] {
			skillcheck.AnalyzeReferences(dir, rpt)
			// If content is disabled, clear the content-specific reference fields
			if !opts.Enabled[GroupContent] {
				rpt.ReferencesContentReport = nil
				for i := range rpt.ReferenceReports {
					rpt.ReferenceReports[i].ContentReport = nil
				}
			}
			// If contamination is disabled, clear the contamination-specific reference fields
			if !opts.Enabled[GroupContamination] {
				rpt.ReferencesContaminationReport = nil
				for i := range rpt.ReferenceReports {
					rpt.ReferenceReports[i].ContaminationReport = nil
				}
			}
		}
	}

	rpt.Tally()
	return rpt
}

// RunContentAnalysis runs content quality analysis on a single skill directory.
func RunContentAnalysis(dir string) *types.Report {
	rpt := &types.Report{SkillDir: dir}

	s, err := skill.Load(dir)
	if err != nil {
		rpt.Results = append(rpt.Results,
			types.ResultContext{Category: "Content"}.Error(err.Error()))
		rpt.Errors = 1
		return rpt
	}

	rpt.ContentReport = content.Analyze(s.RawContent)
	rpt.Results = append(rpt.Results,
		types.ResultContext{Category: "Content"}.Pass("content analysis complete"))

	skillcheck.AnalyzeReferences(dir, rpt)

	return rpt
}

// RunContaminationAnalysis runs cross-language contamination analysis on a
// single skill directory.
func RunContaminationAnalysis(dir string) *types.Report {
	rpt := &types.Report{SkillDir: dir}

	s, err := skill.Load(dir)
	if err != nil {
		rpt.Results = append(rpt.Results,
			types.ResultContext{Category: "Contamination"}.Error(err.Error()))
		rpt.Errors = 1
		return rpt
	}

	// Get code languages from content analysis
	cr := content.Analyze(s.RawContent)
	skillName := util.SkillNameFromDir(dir)
	rpt.ContaminationReport = contamination.Analyze(skillName, s.RawContent, cr.CodeLanguages)

	rpt.Results = append(rpt.Results,
		types.ResultContext{Category: "Contamination"}.Pass("contamination analysis complete"))

	skillcheck.AnalyzeReferences(dir, rpt)

	return rpt
}

// RunLinkChecks validates external HTTP/HTTPS links in a single skill directory.
func RunLinkChecks(ctx context.Context, dir string) *types.Report {
	rpt := &types.Report{SkillDir: dir}

	s, err := skill.Load(dir)
	if err != nil {
		rpt.Results = append(rpt.Results,
			types.ResultContext{Category: "Links"}.Error(err.Error()))
		rpt.Errors = 1
		return rpt
	}

	rpt.Results = append(rpt.Results, links.CheckLinks(ctx, dir, s.Body)...)

	// If no results at all, add a pass result
	if len(rpt.Results) == 0 {
		rpt.Results = append(rpt.Results,
			types.ResultContext{Category: "Links"}.Pass("all link checks passed"))
	}

	rpt.Tally()
	return rpt
}
