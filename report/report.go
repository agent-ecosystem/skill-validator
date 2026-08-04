// Package report formats and prints validation and scoring results. It
// supports colored terminal output, GitHub Actions annotations, JSON, and
// Markdown output formats.
package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/agent-ecosystem/skill-validator/types"
	"github.com/agent-ecosystem/skill-validator/util"
)

// Shorthand aliases for ANSI color constants to keep format strings compact.
const (
	colorReset  = util.ColorReset
	colorRed    = util.ColorRed
	colorGreen  = util.ColorGreen
	colorYellow = util.ColorYellow
	colorCyan   = util.ColorCyan
	colorBold   = util.ColorBold
)

// Print writes a human-readable validation report to w. When perFile is true,
// per-file content and contamination analysis sections are included.
func Print(w io.Writer, r *types.Report, perFile bool) {
	_, _ = fmt.Fprintf(w, "\n%sValidating skill: %s%s\n", colorBold, r.SkillDir, colorReset)

	categories, grouped := groupByCategory(r.Results)

	for _, cat := range categories {
		_, _ = fmt.Fprintf(w, "\n%s%s%s\n", colorBold, cat, colorReset)
		for _, res := range grouped[cat] {
			icon, color := formatLevel(res.Level)
			_, _ = fmt.Fprintf(w, "  %s%s %s%s\n", color, icon, res.Message, colorReset)
		}
	}

	// Token counts
	if len(r.TokenCounts) > 0 {
		_, _ = fmt.Fprintf(w, "\n%sTokens%s\n", colorBold, colorReset)

		maxFileLen := len("Total")
		for _, tc := range r.TokenCounts {
			if len(tc.File) > maxFileLen {
				maxFileLen = len(tc.File)
			}
		}

		total := 0
		for _, tc := range r.TokenCounts {
			total += tc.Tokens
			padding := maxFileLen - len(tc.File) + 2
			_, _ = fmt.Fprintf(w, "  %s%s:%s%s%s tokens\n", colorCyan, tc.File, colorReset, strings.Repeat(" ", padding), util.FormatNumber(tc.Tokens))
		}

		separator := strings.Repeat("─", maxFileLen+20)
		_, _ = fmt.Fprintf(w, "  %s\n", separator)
		padding := maxFileLen - len("Total") + 2
		_, _ = fmt.Fprintf(w, "  %sTotal:%s%s%s tokens\n", colorBold, colorReset, strings.Repeat(" ", padding), util.FormatNumber(total))
	}

	// Other files token counts
	if len(r.OtherTokenCounts) > 0 {
		_, _ = fmt.Fprintf(w, "\n%sOther files (outside standard structure)%s\n", colorBold, colorReset)

		maxFileLen := len("Total (other)")
		for _, tc := range r.OtherTokenCounts {
			if len(tc.File) > maxFileLen {
				maxFileLen = len(tc.File)
			}
		}

		total := 0
		for _, tc := range r.OtherTokenCounts {
			total += tc.Tokens
			padding := maxFileLen - len(tc.File) + 2
			countColor := ""
			countColorEnd := ""
			if tc.Tokens > 25_000 {
				countColor = colorRed
				countColorEnd = colorReset
			} else if tc.Tokens > 10_000 {
				countColor = colorYellow
				countColorEnd = colorReset
			}
			_, _ = fmt.Fprintf(w, "  %s%s:%s%s%s%s tokens%s\n", colorCyan, tc.File, colorReset, strings.Repeat(" ", padding), countColor, util.FormatNumber(tc.Tokens), countColorEnd)
		}

		separator := strings.Repeat("─", maxFileLen+20)
		_, _ = fmt.Fprintf(w, "  %s\n", separator)
		label := "Total (other)"
		padding := maxFileLen - len(label) + 2
		totalColor := ""
		totalColorEnd := ""
		if total > 100_000 {
			totalColor = colorRed
			totalColorEnd = colorReset
		} else if total > 25_000 {
			totalColor = colorYellow
			totalColorEnd = colorReset
		}
		_, _ = fmt.Fprintf(w, "  %s%s:%s%s%s%s tokens%s\n", colorBold, label, colorReset, strings.Repeat(" ", padding), totalColor, util.FormatNumber(total), totalColorEnd)
	}

	// Content analysis
	if r.ContentReport != nil {
		printContentReport(w, "Content Analysis", r.ContentReport)
	}

	// References content analysis
	if r.ReferencesContentReport != nil {
		printContentReport(w, "References Content Analysis", r.ReferencesContentReport)
	}

	// Per-file content analysis
	if perFile && len(r.ReferenceReports) > 0 {
		for _, fr := range r.ReferenceReports {
			if fr.ContentReport != nil {
				printContentReport(w, fmt.Sprintf("  [%s] Content Analysis", fr.File), fr.ContentReport)
			}
		}
	}

	// Contamination analysis
	if r.ContaminationReport != nil {
		printContaminationReport(w, "Contamination Analysis", r.ContaminationReport)
	}

	// References contamination analysis
	if r.ReferencesContaminationReport != nil {
		printContaminationReport(w, "References Contamination Analysis", r.ReferencesContaminationReport)
	}

	// Per-file contamination analysis
	if perFile && len(r.ReferenceReports) > 0 {
		for _, fr := range r.ReferenceReports {
			if fr.ContaminationReport != nil {
				printContaminationReport(w, fmt.Sprintf("  [%s] Contamination Analysis", fr.File), fr.ContaminationReport)
			}
		}
	}

	// Summary
	_, _ = fmt.Fprintln(w)
	if r.Errors == 0 && r.Warnings == 0 {
		_, _ = fmt.Fprintf(w, "%s%sResult: passed%s\n", colorBold, colorGreen, colorReset)
	} else {
		parts := []string{}
		if r.Errors > 0 {
			parts = append(parts, fmt.Sprintf("%s%d error%s%s", colorRed, r.Errors, util.PluralS(r.Errors), colorReset))
		}
		if r.Warnings > 0 {
			parts = append(parts, fmt.Sprintf("%s%d warning%s%s", colorYellow, r.Warnings, util.PluralS(r.Warnings), colorReset))
		}
		_, _ = fmt.Fprintf(w, "%sResult: %s%s\n", colorBold, strings.Join(parts, ", "), colorReset)
	}
	_, _ = fmt.Fprintln(w)
}

// PrintMulti prints each skill report separated by a line, with an overall summary.
func PrintMulti(w io.Writer, mr *types.MultiReport, perFile bool) {
	for i, r := range mr.Skills {
		if i > 0 {
			_, _ = fmt.Fprintf(w, "\n%s\n", strings.Repeat("━", 60))
		}
		Print(w, r, perFile)
	}

	_, _ = fmt.Fprintf(w, "%s\n", strings.Repeat("━", 60))
	printMultiSummary(w, mr)
}

// printMultiSummary writes the trailing per-skill tally and total counts shared
// by the full and compact multi-skill renderers.
func printMultiSummary(w io.Writer, mr *types.MultiReport) {
	passed := 0
	failed := 0
	for _, r := range mr.Skills {
		if r.Errors == 0 {
			passed++
		} else {
			failed++
		}
	}

	_, _ = fmt.Fprintf(w, "\n%s%d skill%s validated: ", colorBold, len(mr.Skills), util.PluralS(len(mr.Skills)))
	if failed == 0 {
		_, _ = fmt.Fprintf(w, "%sall passed%s\n", colorGreen, colorReset)
	} else {
		skillParts := []string{}
		if passed > 0 {
			skillParts = append(skillParts, fmt.Sprintf("%s%d passed%s", colorGreen, passed, colorReset))
		}
		skillParts = append(skillParts, fmt.Sprintf("%s%d failed%s", colorRed, failed, colorReset))
		_, _ = fmt.Fprintf(w, "%s%s\n", strings.Join(skillParts, ", "), colorReset)
	}

	countParts := []string{}
	if mr.Errors > 0 {
		countParts = append(countParts, fmt.Sprintf("%s%d error%s%s", colorRed, mr.Errors, util.PluralS(mr.Errors), colorReset))
	}
	if mr.Warnings > 0 {
		countParts = append(countParts, fmt.Sprintf("%s%d warning%s%s", colorYellow, mr.Warnings, util.PluralS(mr.Warnings), colorReset))
	}
	if len(countParts) > 0 {
		_, _ = fmt.Fprintf(w, "%sTotal: %s%s\n", colorBold, strings.Join(countParts, ", "), colorReset)
	}
	_, _ = fmt.Fprintln(w)
}

func printContentReport(w io.Writer, title string, cr *types.ContentReport) {
	_, _ = fmt.Fprintf(w, "\n%s%s%s\n", colorBold, title, colorReset)
	_, _ = fmt.Fprintf(w, "  Word count:               %s\n", util.FormatNumber(cr.WordCount))
	_, _ = fmt.Fprintf(w, "  Code block ratio:         %.2f\n", cr.CodeBlockRatio)
	_, _ = fmt.Fprintf(w, "  Imperative ratio:         %.2f\n", cr.ImperativeRatio)
	_, _ = fmt.Fprintf(w, "  Information density:      %.2f\n", cr.InformationDensity)
	_, _ = fmt.Fprintf(w, "  Instruction specificity:  %.2f\n", cr.InstructionSpecificity)
	_, _ = fmt.Fprintf(w, "  Sections: %d  |  List items: %d  |  Code blocks: %d\n",
		cr.SectionCount, cr.ListItemCount, cr.CodeBlockCount)
}

func printContaminationReport(w io.Writer, title string, rr *types.ContaminationReport) {
	_, _ = fmt.Fprintf(w, "\n%s%s%s\n", colorBold, title, colorReset)
	levelColor := colorGreen
	switch rr.ContaminationLevel {
	case "high":
		levelColor = colorRed
	case "medium":
		levelColor = colorYellow
	}
	_, _ = fmt.Fprintf(w, "  Contamination level: %s%s%s (score: %.2f)\n", levelColor, rr.ContaminationLevel, colorReset, rr.ContaminationScore)
	if rr.PrimaryCategory != "" {
		_, _ = fmt.Fprintf(w, "  Primary language category: %s\n", rr.PrimaryCategory)
	}
	if rr.LanguageMismatch && len(rr.MismatchedCategories) > 0 {
		_, _ = fmt.Fprintf(w, "  %s⚠ Language mismatch: %s (%d categor%s differ from primary)%s\n",
			colorYellow, strings.Join(rr.MismatchedCategories, ", "),
			len(rr.MismatchedCategories), util.YSuffix(len(rr.MismatchedCategories)), colorReset)
	}
	if len(rr.MultiInterfaceTools) > 0 {
		_, _ = fmt.Fprintf(w, "  %sℹ Multi-interface tool detected: %s%s\n",
			colorCyan, strings.Join(rr.MultiInterfaceTools, ", "), colorReset)
	}
	_, _ = fmt.Fprintf(w, "  Scope breadth: %d\n", rr.ScopeBreadth)
}

// groupByCategory groups results by category, preserving first-appearance order.
func groupByCategory(results []types.Result) ([]string, map[string][]types.Result) {
	var categories []string
	grouped := make(map[string][]types.Result)
	for _, res := range results {
		if _, exists := grouped[res.Category]; !exists {
			categories = append(categories, res.Category)
		}
		grouped[res.Category] = append(grouped[res.Category], res)
	}
	return categories, grouped
}

func formatLevel(level types.Level) (string, string) {
	switch level {
	case types.Pass:
		return "✓", colorGreen
	case types.Info:
		return "ℹ", colorCyan
	case types.Warning:
		return "⚠", colorYellow
	case types.Error:
		return "✗", colorRed
	default:
		return "?", colorReset
	}
}
