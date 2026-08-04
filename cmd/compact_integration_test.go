package cmd_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCompactOutput(t *testing.T) {
	bin := buildBinary(t)

	t.Run("passing skill is a single line", func(t *testing.T) {
		cmd := exec.Command(bin, "check", "-o", "compact", fixture(t, "valid-skill"))
		out, _ := cmd.CombinedOutput()
		s := string(out)

		if got := cmd.ProcessState.ExitCode(); got != 0 {
			t.Errorf("exit code = %d, want 0\noutput: %s", got, s)
		}
		if got := strings.Count(strings.TrimRight(s, "\n"), "\n"); got != 0 {
			t.Errorf("expected a single line, got:\n%s", s)
		}
		if !strings.Contains(s, ": passed") {
			t.Errorf("expected a passed summary, got:\n%s", s)
		}
	})

	t.Run("warnings are listed under the summary", func(t *testing.T) {
		cmd := exec.Command(bin, "check", "-o", "compact", fixture(t, "warnings-only-skill"))
		out, _ := cmd.CombinedOutput()
		s := string(out)

		if got := cmd.ProcessState.ExitCode(); got != 2 {
			t.Errorf("exit code = %d, want 2\noutput: %s", got, s)
		}
		if !strings.Contains(s, "1 warning") {
			t.Errorf("expected the warning count on the summary line:\n%s", s)
		}
		if !strings.Contains(s, "    ") || !strings.Contains(s, "unknown directory") {
			t.Errorf("expected the warning indented beneath the summary:\n%s", s)
		}
		if strings.Contains(s, "Validating skill") || strings.Contains(s, "Tokens") {
			t.Errorf("compact output should omit the full report sections:\n%s", s)
		}
	})

	t.Run("errors are listed under the summary", func(t *testing.T) {
		cmd := exec.Command(bin, "check", "-o", "compact", fixture(t, "invalid-skill"))
		out, _ := cmd.CombinedOutput()
		s := string(out)

		if got := cmd.ProcessState.ExitCode(); got != 1 {
			t.Errorf("exit code = %d, want 1\noutput: %s", got, s)
		}
		if !strings.Contains(s, "error") {
			t.Errorf("expected the error count on the summary line:\n%s", s)
		}
	})

	t.Run("text output is unchanged", func(t *testing.T) {
		cmd := exec.Command(bin, "check", fixture(t, "warnings-only-skill"))
		out, _ := cmd.CombinedOutput()
		s := string(out)

		if !strings.Contains(s, "Validating skill") || !strings.Contains(s, "Tokens") {
			t.Errorf("default text output should still be the full report:\n%s", s)
		}
	})
}
