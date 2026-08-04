package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/agent-ecosystem/skill-validator/types"
)

func TestPrintAnnotations_ErrorAndWarning(t *testing.T) {
	r := &types.Report{
		SkillDir: "/workspace/skills/my-skill",
		Results: []types.Result{
			{Level: types.Error, Category: "Frontmatter", Message: "name is required", File: "SKILL.md"},
			{Level: types.Warning, Category: "Structure", Message: "extraneous file", File: "README.md"},
		},
	}

	var buf bytes.Buffer
	PrintAnnotations(&buf, r, "/workspace")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), buf.String())
	}

	// Path should be relative to workDir
	if !strings.Contains(lines[0], "file=skills/my-skill/SKILL.md") {
		t.Errorf("expected relative path skills/my-skill/SKILL.md, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "title=Frontmatter") {
		t.Errorf("expected title=Frontmatter, got %q", lines[0])
	}
	if !strings.HasSuffix(lines[0], "::name is required") {
		t.Errorf("expected message suffix, got %q", lines[0])
	}

	if !strings.HasPrefix(lines[1], "::warning file=") {
		t.Errorf("expected ::warning prefix, got %q", lines[1])
	}
}

func TestPrintAnnotations_SkipsPassAndInfo(t *testing.T) {
	r := &types.Report{
		SkillDir: "/workspace/skills/my-skill",
		Results: []types.Result{
			{Level: types.Pass, Category: "Structure", Message: "SKILL.md found", File: "SKILL.md"},
			{Level: types.Info, Category: "Links", Message: "HTTP 403", File: "SKILL.md"},
		},
	}

	var buf bytes.Buffer
	PrintAnnotations(&buf, r, "/workspace")

	if buf.Len() != 0 {
		t.Errorf("expected no output for Pass/Info, got %q", buf.String())
	}
}

func TestPrintAnnotations_WithLineNumber(t *testing.T) {
	r := &types.Report{
		SkillDir: "/workspace/skills/my-skill",
		Results: []types.Result{
			{Level: types.Error, Category: "Markdown", Message: "unclosed fence", File: "SKILL.md", Line: 42},
		},
	}

	var buf bytes.Buffer
	PrintAnnotations(&buf, r, "/workspace")

	line := strings.TrimSpace(buf.String())
	if !strings.Contains(line, "file=skills/my-skill/SKILL.md") {
		t.Errorf("expected relative path, got %q", line)
	}
	if !strings.Contains(line, "line=42") {
		t.Errorf("expected line=42, got %q", line)
	}
}

func TestPrintAnnotations_NoFile(t *testing.T) {
	r := &types.Report{
		SkillDir: "skills/my-skill",
		Results: []types.Result{
			{Level: types.Error, Category: "Overall", Message: "not a skill"},
		},
	}

	var buf bytes.Buffer
	PrintAnnotations(&buf, r, ".")

	line := strings.TrimSpace(buf.String())
	expected := "::error title=Overall::not a skill"
	if line != expected {
		t.Errorf("expected %q, got %q", expected, line)
	}
}

func TestPrintAnnotations_EscapesInjection(t *testing.T) {
	r := &types.Report{
		SkillDir: "/workspace/skills/my-skill",
		Results: []types.Result{
			{
				Level:    types.Error,
				Category: "Front,matter:bad",
				Message:  "evil\n::error file=/etc/passwd::PWNED\nmore 100% safe?",
				File:     "name\nwith newline.md",
			},
		},
	}

	var buf bytes.Buffer
	PrintAnnotations(&buf, r, "/workspace")

	out := buf.String()
	if got := strings.Count(out, "\n"); got != 1 {
		t.Fatalf("expected exactly one newline (trailing), got %d in %q", got, out)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one annotation line, got %d: %q", len(lines), out)
	}
	for _, l := range lines[1:] {
		if strings.HasPrefix(l, "::") {
			t.Errorf("injected annotation line emitted: %q", l)
		}
	}
	if !strings.Contains(out, "%0A") {
		t.Errorf("newline should be encoded as %%0A, got %q", out)
	}
	if !strings.Contains(out, "%25") {
		t.Errorf("percent should be encoded as %%25, got %q", out)
	}
	if !strings.Contains(out, "Front%2Cmatter%3Abad") {
		t.Errorf("property fields should encode , and :, got %q", out)
	}
}

func TestPrintMultiAnnotations(t *testing.T) {
	mr := &types.MultiReport{
		Skills: []*types.Report{
			{
				SkillDir: "/workspace/skills/a",
				Results: []types.Result{
					{Level: types.Error, Category: "Structure", Message: "missing", File: "SKILL.md"},
				},
			},
			{
				SkillDir: "/workspace/skills/b",
				Results: []types.Result{
					{Level: types.Warning, Category: "Tokens", Message: "too large", File: "references/big.md"},
				},
			},
		},
	}

	var buf bytes.Buffer
	PrintMultiAnnotations(&buf, mr, "/workspace")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "file=skills/a/SKILL.md") {
		t.Errorf("expected skills/a/SKILL.md path, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "file=skills/b/references/big.md") {
		t.Errorf("expected skills/b/references/big.md path, got %q", lines[1])
	}
}
