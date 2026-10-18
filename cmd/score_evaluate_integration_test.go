package cmd_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestScoreEvaluateResponseBudgetFlag(t *testing.T) {
	bin := buildBinary(t)

	t.Run("listed in help", func(t *testing.T) {
		out, _ := exec.Command(bin, "score", "evaluate", "--help").CombinedOutput()
		if !strings.Contains(string(out), "--max-response-tokens") {
			t.Errorf("help does not mention --max-response-tokens:\n%s", out)
		}
	})

	t.Run("rejects zero before needing a key", func(t *testing.T) {
		out, _ := exec.Command(bin, "score", "evaluate", "--max-response-tokens", "0", fixture(t, "valid-skill")).CombinedOutput()
		if !strings.Contains(string(out), "--max-response-tokens must be at least 1") {
			t.Errorf("unexpected output:\n%s", out)
		}
	})
}
