//go:build windows

package cmd_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/agent-ecosystem/skill-validator/internal/testutil"
)

// copyTree copies the regular files under src to dst.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Issue #101 end to end: a skills directory made of junctions, the layout
// that was reported, must validate every skill rather than silently only
// the ones that are real directories.
func TestCheckJunctionedCatalog(t *testing.T) {
	bin := buildBinary(t)
	valid := fixture(t, "valid-skill")

	source := t.TempDir()
	catalog := t.TempDir()
	names := []string{"alpha", "beta", "gamma"}
	for _, name := range names {
		copyTree(t, valid, filepath.Join(source, name))
		testutil.Junction(t, filepath.Join(source, name), filepath.Join(catalog, name))
	}
	copyTree(t, valid, filepath.Join(catalog, "plain"))
	wantSkills := len(names) + 1

	out, err := exec.Command(bin, "check", "-o", "json", catalog).Output()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Fatalf("running check: %v", err)
	}

	var report struct {
		Skills []struct {
			SkillDir string `json:"skill_dir"`
			Errors   int    `json:"errors"`
			Results  []struct {
				Level   string `json:"level"`
				Message string `json:"message"`
			} `json:"results"`
		} `json:"skills"`
	}
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("parsing report: %v\n%s", err, out)
	}
	if len(report.Skills) != wantSkills {
		var dirs []string
		for _, s := range report.Skills {
			dirs = append(dirs, s.SkillDir)
		}
		t.Fatalf("validated %d skills %v, want %d", len(report.Skills), dirs, wantSkills)
	}
	for _, s := range report.Skills {
		if s.Errors == 0 {
			continue
		}
		t.Errorf("%s: %d errors", s.SkillDir, s.Errors)
		for _, r := range s.Results {
			if r.Level == "error" {
				t.Logf("  %s", r.Message)
			}
		}
	}
}
