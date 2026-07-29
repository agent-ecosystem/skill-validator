package report

import (
	"strings"
	"testing"

	"github.com/agent-ecosystem/skill-validator/types"
)

func compactFixture(dir string, results ...types.Result) *types.Report {
	r := &types.Report{
		SkillDir:            dir,
		Results:             results,
		TokenCounts:         []types.TokenCount{{File: "SKILL.md", Tokens: 100}},
		ContentReport:       &types.ContentReport{WordCount: 10},
		ContaminationReport: &types.ContaminationReport{ContaminationLevel: "low"},
	}
	r.Tally()
	return r
}

func TestPrintCompact_Passed(t *testing.T) {
	r := compactFixture("my-skill",
		types.Result{Level: types.Pass, Category: "Structure", Message: "SKILL.md found"},
		types.Result{Level: types.Info, Category: "Structure", Message: "fyi"},
	)

	var buf strings.Builder
	PrintCompact(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "✓ my-skill: passed") {
		t.Errorf("expected one-line passed summary, got:\n%s", out)
	}
	if got := strings.Count(out, "\n"); got != 1 {
		t.Errorf("a passing skill should occupy one line, got %d:\n%s", got, out)
	}
	if strings.Contains(out, "SKILL.md found") || strings.Contains(out, "fyi") {
		t.Errorf("pass and info findings should be omitted:\n%s", out)
	}
}

func TestPrintCompact_ListsWarningsAndErrors(t *testing.T) {
	r := compactFixture("my-skill",
		types.Result{Level: types.Pass, Category: "Structure", Message: "SKILL.md found"},
		types.Result{Level: types.Error, Category: "Links", Message: "broken link: ./gone.md"},
		types.Result{Level: types.Warning, Category: "Structure", Message: "unknown directory: extras/"},
	)

	var buf strings.Builder
	PrintCompact(&buf, r)
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")

	if len(lines) != 3 {
		t.Fatalf("expected a summary line plus two findings, got %d lines:\n%s", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], "✗ my-skill: ") ||
		!strings.Contains(lines[0], "1 error") || !strings.Contains(lines[0], "1 warning") {
		t.Errorf("summary line should carry both counts, got: %s", lines[0])
	}
	for _, line := range lines[1:] {
		if !strings.HasPrefix(line, "    ") {
			t.Errorf("findings should be indented under the summary, got: %q", line)
		}
	}
	if !strings.Contains(lines[1], "broken link: ./gone.md") {
		t.Errorf("expected the error message, got: %s", lines[1])
	}
	if !strings.Contains(lines[2], "unknown directory: extras/") {
		t.Errorf("expected the warning message, got: %s", lines[2])
	}
	if strings.Contains(buf.String(), "SKILL.md found") {
		t.Errorf("pass findings should be omitted:\n%s", buf.String())
	}
}

func TestPrintCompact_OmitsAnalysisSections(t *testing.T) {
	r := compactFixture("my-skill",
		types.Result{Level: types.Warning, Category: "Structure", Message: "meh"},
	)
	r.OtherTokenCounts = []types.TokenCount{{File: "extra.md", Tokens: 50}}

	var buf strings.Builder
	PrintCompact(&buf, r)
	out := buf.String()

	for _, unwanted := range []string{"Tokens", "Content Analysis", "Contamination Analysis", "Validating skill"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("compact output should omit %q:\n%s", unwanted, out)
		}
	}
}

func TestPrintCompact_WarningOnlyUsesWarningIcon(t *testing.T) {
	r := compactFixture("my-skill",
		types.Result{Level: types.Warning, Category: "Structure", Message: "meh"},
	)

	var buf strings.Builder
	PrintCompact(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "⚠ my-skill: 1 warning") {
		t.Errorf("expected warning icon and count, got:\n%s", out)
	}
	if strings.Contains(out, "✗") {
		t.Errorf("a warning-only report should not use the error icon:\n%s", out)
	}
}

func TestPrintMultiCompact(t *testing.T) {
	mr := &types.MultiReport{
		Skills: []*types.Report{
			compactFixture("a"),
			compactFixture("b", types.Result{Level: types.Warning, Category: "Structure", Message: "meh"}),
			compactFixture("c"),
		},
	}
	for _, r := range mr.Skills {
		mr.Errors += r.Errors
		mr.Warnings += r.Warnings
	}

	var buf strings.Builder
	PrintMultiCompact(&buf, mr)
	out := buf.String()

	if !strings.Contains(out, "✓ a: passed") || !strings.Contains(out, "✓ c: passed") {
		t.Errorf("expected one-line summaries for passing skills, got:\n%s", out)
	}
	if !strings.Contains(out, "⚠ b: 1 warning") || !strings.Contains(out, "    ") {
		t.Errorf("expected the failing skill's finding indented, got:\n%s", out)
	}
	if strings.Contains(out, strings.Repeat("━", 60)) {
		t.Errorf("compact output should not draw separators:\n%s", out)
	}
	if !strings.Contains(out, "3 skills validated") || !strings.Contains(out, "Total: ") {
		t.Errorf("expected the overall summary, got:\n%s", out)
	}
}
