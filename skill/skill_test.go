package skill

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/agent-ecosystem/skill-validator/util"
)

func TestSplitFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantFM   string
		wantBody string
		wantErr  bool
	}{
		{
			name:     "valid frontmatter and body",
			input:    "---\nname: test\n---\n# Hello\n",
			wantFM:   "name: test",
			wantBody: "# Hello\n",
		},
		{
			name:     "no frontmatter",
			input:    "# Just a body\nSome text.\n",
			wantFM:   "",
			wantBody: "# Just a body\nSome text.\n",
		},
		{
			name:    "unterminated frontmatter",
			input:   "---\nname: test\n# No closing delimiter\n",
			wantErr: true,
		},
		{
			name:     "empty frontmatter",
			input:    "---\n---\n# Body\n",
			wantFM:   "",
			wantBody: "# Body\n",
		},
		{
			name:     "frontmatter with multiple fields",
			input:    "---\nname: test\ndescription: A test\nlicense: MIT\n---\nBody here.\n",
			wantFM:   "name: test\ndescription: A test\nlicense: MIT",
			wantBody: "Body here.\n",
		},
		{
			name:     "body with triple dashes inside",
			input:    "---\nname: test\n---\nSome text\n---\nMore text\n",
			wantFM:   "name: test",
			wantBody: "Some text\n---\nMore text\n",
		},
		{
			name:     "windows line endings",
			input:    "---\r\nname: test\r\n---\r\nBody\r\n",
			wantFM:   "name: test",
			wantBody: "Body\r\n",
		},
		{
			name:     "empty frontmatter with windows line endings",
			input:    "---\r\n---\r\nBody\r\n",
			wantFM:   "",
			wantBody: "Body\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body, err := splitFrontmatter(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if fm != tt.wantFM {
				t.Errorf("frontmatter = %q, want %q", fm, tt.wantFM)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	t.Run("valid skill", func(t *testing.T) {
		dir := t.TempDir()
		content := "---\nname: my-skill\ndescription: A test skill\nlicense: MIT\n---\n# My Skill\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		s, err := Load(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Frontmatter.Name != "my-skill" {
			t.Errorf("name = %q, want %q", s.Frontmatter.Name, "my-skill")
		}
		if s.Frontmatter.Description != "A test skill" {
			t.Errorf("description = %q, want %q", s.Frontmatter.Description, "A test skill")
		}
		if s.Frontmatter.License != "MIT" {
			t.Errorf("license = %q, want %q", s.Frontmatter.License, "MIT")
		}
		if s.Body != "# My Skill\n" {
			t.Errorf("body = %q, want %q", s.Body, "# My Skill\n")
		}
		if s.Dir != dir {
			t.Errorf("dir = %q, want %q", s.Dir, dir)
		}
	})

	t.Run("missing SKILL.md", func(t *testing.T) {
		dir := t.TempDir()
		_, err := Load(dir)
		if err == nil {
			t.Fatal("expected error for missing SKILL.md")
		}
	})

	t.Run("refuses SKILL.md symlink", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks require admin on Windows")
		}
		dir := t.TempDir()
		secret := filepath.Join(dir, "secret.md")
		if err := os.WriteFile(secret, []byte("---\nname: x\ndescription: x\n---\nLEAK"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(secret, filepath.Join(dir, "SKILL.md")); err != nil {
			t.Fatal(err)
		}
		_, err := Load(dir)
		if !errors.Is(err, util.ErrUnsafeFile) {
			t.Fatalf("expected ErrUnsafeFile, got %v", err)
		}
	})

	t.Run("invalid YAML", func(t *testing.T) {
		dir := t.TempDir()
		content := "---\n: invalid: yaml: [broken\n---\nBody\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := Load(dir)
		if err == nil {
			t.Fatal("expected error for invalid YAML")
		}
	})

	t.Run("unterminated frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		content := "---\nname: test\n# No closing delimiter\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := Load(dir)
		if err == nil {
			t.Fatal("expected error for unterminated frontmatter")
		}
	})

	t.Run("no frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		content := "# Just a body\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		s, err := Load(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Frontmatter.Name != "" {
			t.Errorf("name should be empty, got %q", s.Frontmatter.Name)
		}
		if s.Body != "# Just a body\n" {
			t.Errorf("body = %q, want %q", s.Body, "# Just a body\n")
		}
	})

	t.Run("allowed-tools as string", func(t *testing.T) {
		dir := t.TempDir()
		content := "---\nname: my-skill\ndescription: A test skill\nallowed-tools: Read Write Bash\n---\n# My Skill\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		s, err := Load(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Frontmatter.AllowedTools.Value != "Read Write Bash" {
			t.Errorf("allowed-tools value = %q, want %q", s.Frontmatter.AllowedTools.Value, "Read Write Bash")
		}
		if s.Frontmatter.AllowedTools.WasList {
			t.Error("expected WasList=false for string format")
		}
	})

	t.Run("allowed-tools as inline list", func(t *testing.T) {
		dir := t.TempDir()
		content := "---\nname: my-skill\ndescription: A test skill\nallowed-tools: [Read, Write, Bash]\n---\n# My Skill\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		s, err := Load(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Frontmatter.AllowedTools.Value != "Read Write Bash" {
			t.Errorf("allowed-tools value = %q, want %q", s.Frontmatter.AllowedTools.Value, "Read Write Bash")
		}
		if !s.Frontmatter.AllowedTools.WasList {
			t.Error("expected WasList=true for inline list format")
		}
	})

	t.Run("allowed-tools as block list", func(t *testing.T) {
		dir := t.TempDir()
		content := "---\nname: my-skill\ndescription: A test skill\nallowed-tools:\n  - Read\n  - Bash\n  - Grep\n---\n# My Skill\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		s, err := Load(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Frontmatter.AllowedTools.Value != "Read Bash Grep" {
			t.Errorf("allowed-tools value = %q, want %q", s.Frontmatter.AllowedTools.Value, "Read Bash Grep")
		}
		if !s.Frontmatter.AllowedTools.WasList {
			t.Error("expected WasList=true for block list format")
		}
	})

	t.Run("allowed-tools absent", func(t *testing.T) {
		dir := t.TempDir()
		content := "---\nname: my-skill\ndescription: A test skill\n---\n# My Skill\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		s, err := Load(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !s.Frontmatter.AllowedTools.IsEmpty() {
			t.Errorf("expected empty allowed-tools, got %q", s.Frontmatter.AllowedTools.Value)
		}
	})

	t.Run("metadata parsing", func(t *testing.T) {
		dir := t.TempDir()
		content := "---\nname: test\ndescription: desc\nmetadata:\n  author: alice\n  version: \"1.0\"\n---\nBody\n"
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		s, err := Load(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.Frontmatter.Metadata["author"] != "alice" {
			t.Errorf("metadata[author] = %q, want %q", s.Frontmatter.Metadata["author"], "alice")
		}
		if s.Frontmatter.Metadata["version"] != "1.0" {
			t.Errorf("metadata[version] = %q, want %q", s.Frontmatter.Metadata["version"], "1.0")
		}
	})
}

func TestUnrecognizedFields(t *testing.T) {
	dir := t.TempDir()
	content := "---\nname: test\ndescription: desc\ncustom-field: value\nanother: thing\n---\nBody\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	unknown := s.UnrecognizedFields()
	if len(unknown) != 2 {
		t.Fatalf("expected 2 unrecognized fields, got %d: %v", len(unknown), unknown)
	}
	found := map[string]bool{}
	for _, f := range unknown {
		found[f] = true
	}
	if !found["custom-field"] {
		t.Error("expected custom-field in unrecognized fields")
	}
	if !found["another"] {
		t.Error("expected another in unrecognized fields")
	}
}
