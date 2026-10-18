//go:build windows

package util

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agent-ecosystem/skill-validator/internal/testutil"
)

func TestRealPath_Junction(t *testing.T) {
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "sub", "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	testutil.Junction(t, source, link)

	// filepath.EvalSymlinks alone fails here with "not a directory" since
	// Go 1.23, because the junction above file.txt is neither a directory
	// nor a symlink to it.
	got, err := RealPath(filepath.Join(link, "sub", "file.txt"))
	if err != nil {
		t.Fatalf("RealPath through junction: %v", err)
	}
	want, err := filepath.EvalSymlinks(filepath.Join(source, "sub", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("RealPath = %q, want %q", got, want)
	}
}

// A skill reached through a junction (#101) must read like any other: the
// root and the files under it resolve through the same junction.
func TestSafeReadFile_JunctionedRoot(t *testing.T) {
	source := t.TempDir()
	files := map[string]string{
		"SKILL.md":                              "skill",
		filepath.Join("references", "guide.md"): "guide",
	}
	for rel, content := range files {
		path := filepath.Join(source, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join(t.TempDir(), "skill")
	testutil.Junction(t, source, root)

	for rel, want := range files {
		got, err := SafeReadFile(root, filepath.Join(root, rel))
		if err != nil {
			t.Errorf("SafeReadFile(%s): %v", rel, err)
			continue
		}
		if string(got) != want {
			t.Errorf("SafeReadFile(%s) = %q, want %q", rel, got, want)
		}
	}
}

// Issue #88 for junctions: a junction inside the skill that leads outside it
// must be refused, just as a symlinked directory is.
func TestSafeReadFile_JunctionEscapesRoot(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("OUT-OF-TREE"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	testutil.Junction(t, outside, filepath.Join(root, "references"))

	leaked := filepath.Join(root, "references", "secret.md")
	if _, err := SafeReadFile(root, leaked); !errors.Is(err, ErrUnsafeFile) {
		t.Errorf("SafeReadFile through escaping junction error = %v, want ErrUnsafeFile", err)
	}
}
