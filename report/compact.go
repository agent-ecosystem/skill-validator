package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/agent-ecosystem/skill-validator/types"
	"github.com/agent-ecosystem/skill-validator/util"
)

// PrintCompact writes a one-line outcome for the skill. Warnings and errors are
// listed underneath it; passing checks, token counts, and analysis sections are
// omitted.
func PrintCompact(w io.Writer, r *types.Report) {
	printCompactSkill(w, r)
}

// PrintMultiCompact writes one compact entry per skill, followed by the overall
// summary.
func PrintMultiCompact(w io.Writer, mr *types.MultiReport) {
	for _, r := range mr.Skills {
		printCompactSkill(w, r)
	}
	printMultiSummary(w, mr)
}

func printCompactSkill(w io.Writer, r *types.Report) {
	icon, color := compactLevel(r)
	_, _ = fmt.Fprintf(w, "%s%s %s: %s%s\n", color, icon, r.SkillDir, compactOutcome(r), colorReset)

	for _, res := range r.Results {
		if res.Level < types.Warning {
			continue
		}
		resIcon, resColor := formatLevel(res.Level)
		_, _ = fmt.Fprintf(w, "    %s%s %s%s\n", resColor, resIcon, res.Message, colorReset)
	}
}

// compactLevel returns the icon and color representing the report's worst outcome.
func compactLevel(r *types.Report) (string, string) {
	switch {
	case r.Errors > 0:
		return "✗", colorRed
	case r.Warnings > 0:
		return "⚠", colorYellow
	default:
		return "✓", colorGreen
	}
}

// compactOutcome summarizes the report as "passed" or as its finding counts.
func compactOutcome(r *types.Report) string {
	if r.Errors == 0 && r.Warnings == 0 {
		return "passed"
	}

	parts := []string{}
	if r.Errors > 0 {
		parts = append(parts, fmt.Sprintf("%d error%s", r.Errors, util.PluralS(r.Errors)))
	}
	if r.Warnings > 0 {
		parts = append(parts, fmt.Sprintf("%d warning%s", r.Warnings, util.PluralS(r.Warnings)))
	}
	return strings.Join(parts, ", ")
}
