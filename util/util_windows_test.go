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

// Links of both kinds can chain: a symlink may lead into a junction and a
// junction may lead to a symlink. Each element is resolved in turn.
func TestRealPath_MixedLinkChains(t *testing.T) {
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "sub", "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(filepath.Join(source, "sub", "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	junction := filepath.Join(base, "junction")
	testutil.Junction(t, source, junction)

	check := func(t *testing.T, path string) {
		t.Helper()
		got, err := RealPath(path)
		if err != nil {
			t.Fatalf("RealPath(%s): %v", path, err)
		}
		if got != want {
			t.Errorf("RealPath(%s) = %q, want %q", path, got, want)
		}
	}

	t.Run("symlink into a junction", func(t *testing.T) {
		link := filepath.Join(base, "via-symlink")
		testutil.Symlink(t, filepath.Join(junction, "sub"), link)
		check(t, filepath.Join(link, "file.txt"))
	})

	t.Run("relative symlink into a junction", func(t *testing.T) {
		link := filepath.Join(base, "via-relative")
		testutil.Symlink(t, filepath.Join("junction", "sub"), link)
		check(t, filepath.Join(link, "file.txt"))
	})

	t.Run("junction to a symlink", func(t *testing.T) {
		link := filepath.Join(base, "symlinked-source")
		testutil.Symlink(t, source, link)
		second := filepath.Join(base, "junction-to-symlink")
		testutil.Junction(t, link, second)
		check(t, filepath.Join(second, "sub", "file.txt"))
	})

	t.Run("reads through a symlink into a junction", func(t *testing.T) {
		root := filepath.Join(base, "skill-alias")
		testutil.Symlink(t, filepath.Join(junction, "sub"), root)
		got, err := SafeReadFile(root, filepath.Join(root, "file.txt"))
		if err != nil {
			t.Fatalf("SafeReadFile: %v", err)
		}
		if string(got) != "x" {
			t.Errorf("SafeReadFile = %q, want %q", got, "x")
		}
	})
}
