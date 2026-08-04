package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildBinary compiles the CLI to a temp directory and returns the path.
func buildBinary(t *testing.T) string {
	t.Helper()
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	bin := filepath.Join(t.TempDir(), "skill-validator"+ext)
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/skill-validator")
	cmd.Dir = moduleRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

// moduleRoot returns the project root (parent of cmd/).
func moduleRoot(t *testing.T) string {
	t.Helper()
	// This file lives in cmd/, so the module root is one level up.
	dir, err := filepath.Abs(filepath.Join("..", "."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatalf("cannot find module root: %v", err)
	}
	return dir
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(moduleRoot(t), "testdata", name)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("fixture %q not found: %v", name, err)
	}
	return p
}

func TestExitCodes(t *testing.T) {
	bin := buildBinary(t)

	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{
			name:     "clean skill exits 0",
			args:     []string{"check", fixture(t, "valid-skill")},
			wantCode: 0,
		},
		{
			name:     "errors exit 1",
			args:     []string{"check", fixture(t, "invalid-skill")},
			wantCode: 1,
		},
		{
			name:     "warnings-only exits 2",
			args:     []string{"check", fixture(t, "warnings-only-skill")},
			wantCode: 2,
		},
		{
			name:     "strict with warnings exits 1",
			args:     []string{"check", "--strict", fixture(t, "warnings-only-skill")},
			wantCode: 1,
		},
		{
			name:     "bad flag exits 3",
			args:     []string{"check", "--bogus"},
			wantCode: 3,
		},
		{
			name:     "validate structure strict with warnings exits 1",
			args:     []string{"validate", "structure", "--strict", fixture(t, "warnings-only-skill")},
			wantCode: 1,
		},
		{
			name:     "validate structure warnings-only exits 2",
			args:     []string{"validate", "structure", fixture(t, "warnings-only-skill")},
			wantCode: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(bin, tt.args...)
			_ = cmd.Run()
			got := cmd.ProcessState.ExitCode()
			if got != tt.wantCode {
				t.Errorf("exit code = %d, want %d (args: %v)", got, tt.wantCode, tt.args)
			}
		})
	}
}

func TestSliceFlags(t *testing.T) {
	bin := buildBinary(t)

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string // substring that must appear in combined output
		noStdout   string // substring that must NOT appear in combined output
	}{
		// --only: comma-separated
		{
			name:       "only comma-separated runs selected groups",
			args:       []string{"check", "--only=structure,content", fixture(t, "valid-skill")},
			wantCode:   0,
			wantStdout: "SKILL.md found",
		},
		// --only: repeated flag
		{
			name:       "only repeated flag runs selected groups",
			args:       []string{"check", "--only=structure", "--only=content", fixture(t, "valid-skill")},
			wantCode:   0,
			wantStdout: "SKILL.md found",
		},
		// --skip: comma-separated
		{
			name:       "skip comma-separated excludes groups",
			args:       []string{"check", "--skip=links,content,contamination", fixture(t, "valid-skill")},
			wantCode:   0,
			wantStdout: "SKILL.md found",
		},
		// --skip: repeated flag
		{
			name:       "skip repeated flag excludes groups",
			args:       []string{"check", "--skip=links", "--skip=content", "--skip=contamination", fixture(t, "valid-skill")},
			wantCode:   0,
			wantStdout: "SKILL.md found",
		},
		// --only and --skip mutual exclusion
		{
			name:     "only and skip mutual exclusion",
			args:     []string{"check", "--only=structure", "--skip=links", fixture(t, "valid-skill")},
			wantCode: 3,
		},
		// --allow-dirs: comma-separated
		{
			name:     "allow-dirs comma-separated suppresses warnings",
			args:     []string{"check", "--only=structure", "--allow-dirs=evals,testing", fixture(t, "allowed-dirs-skill")},
			wantCode: 0,
			noStdout: "unknown directory",
		},
		// --allow-dirs: repeated flag
		{
			name:     "allow-dirs repeated flag suppresses warnings",
			args:     []string{"check", "--only=structure", "--allow-dirs=evals", "--allow-dirs=testing", fixture(t, "allowed-dirs-skill")},
			wantCode: 0,
			noStdout: "unknown directory",
		},
		// --allow-dirs: partial (only one of two unknown dirs)
		{
			name:       "allow-dirs partial still warns for non-allowed",
			args:       []string{"check", "--only=structure", "--allow-dirs=evals", fixture(t, "allowed-dirs-skill")},
			wantCode:   2,
			wantStdout: "unknown directory: testing/",
			noStdout:   "unknown directory: evals/",
		},
		{
			name:       "check allow-nested-paths suppresses exact path only",
			args:       []string{"check", "--only=structure", "--skip-orphans", "--allow-nested-paths=assets/components", fixture(t, "nested-paths-skill")},
			wantCode:   2,
			wantStdout: "deep nesting detected: assets/components-extra/",
			noStdout:   "deep nesting detected: assets/components/",
		},
		{
			name:       "validate structure allow-nested-paths accepts Windows separators",
			args:       []string{"validate", "structure", "--skip-orphans", `--allow-nested-paths=assets\components,assets\components-extra`, fixture(t, "nested-paths-skill")},
			wantCode:   0,
			wantStdout: "deep nesting allowed: assets/components/",
			noStdout:   "deep nesting detected:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(bin, tt.args...)
			out, _ := cmd.CombinedOutput()
			got := cmd.ProcessState.ExitCode()
			if got != tt.wantCode {
				t.Errorf("exit code = %d, want %d (args: %v)\noutput: %s", got, tt.wantCode, tt.args, out)
			}
			if tt.wantStdout != "" {
				if !strings.Contains(string(out), tt.wantStdout) {
					t.Errorf("expected output to contain %q, got:\n%s", tt.wantStdout, out)
				}
			}
			if tt.noStdout != "" {
				if strings.Contains(string(out), tt.noStdout) {
					t.Errorf("expected output NOT to contain %q, got:\n%s", tt.noStdout, out)
				}
			}
		})
	}
}

func TestAllowNestedPathsInvalidPath(t *testing.T) {
	bin := buildBinary(t)

	for _, args := range [][]string{
		{"validate", "structure", "--allow-nested-paths=/assets/components", fixture(t, "valid-skill")},
		{"check", "--allow-nested-paths=../components", fixture(t, "valid-skill")},
	} {
		cmd := exec.Command(bin, args...)
		out, _ := cmd.CombinedOutput()
		if got := cmd.ProcessState.ExitCode(); got != 3 {
			t.Errorf("exit code = %d, want 3 (args: %v)\noutput: %s", got, args, out)
		}
		if !strings.Contains(string(out), "invalid --allow-nested-paths") {
			t.Errorf("expected invalid path error, got:\n%s", out)
		}
	}
}

func TestAllowNestedPathsOutputFormats(t *testing.T) {
	bin := buildBinary(t)

	for _, format := range []string{"text", "json", "markdown"} {
		t.Run(format, func(t *testing.T) {
			args := []string{
				"validate", "structure",
				"--skip-orphans",
				"--allow-nested-paths=assets/components,assets/components-extra",
				"--output=" + format,
				fixture(t, "nested-paths-skill"),
			}
			cmd := exec.Command(bin, args...)
			out, _ := cmd.CombinedOutput()

			if got := cmd.ProcessState.ExitCode(); got != 0 {
				t.Errorf("exit code = %d, want 0\noutput: %s", got, out)
			}
			if !strings.Contains(string(out), "deep nesting allowed: assets/components/") {
				t.Errorf("expected allowed-path info in %s output, got:\n%s", format, out)
			}
			if strings.Contains(string(out), "deep nesting detected:") {
				t.Errorf("unexpected deep-nesting warning in %s output:\n%s", format, out)
			}
		})
	}
}
