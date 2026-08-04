package util

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRoundTo(t *testing.T) {
	tests := []struct {
		val    float64
		places int
		want   float64
	}{
		{0.12345, 4, 0.1235},
		{0.5, 2, 0.5},
		{1.0, 4, 1.0},
		{0.0, 4, 0.0},
	}
	for _, tt := range tests {
		got := RoundTo(tt.val, tt.places)
		if math.Abs(got-tt.want) > 1e-10 {
			t.Errorf("RoundTo(%f, %d) = %f, want %f", tt.val, tt.places, got, tt.want)
		}
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{12345, "12,345"},
		{1000000, "1,000,000"},
	}
	for _, tt := range tests {
		got := FormatNumber(tt.n)
		if got != tt.want {
			t.Errorf("FormatNumber(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestPluralS(t *testing.T) {
	if PluralS(1) != "" {
		t.Error("PluralS(1) should be empty")
	}
	if PluralS(0) != "s" {
		t.Error("PluralS(0) should be 's'")
	}
	if PluralS(2) != "s" {
		t.Error("PluralS(2) should be 's'")
	}
}

func TestYSuffix(t *testing.T) {
	if YSuffix(1) != "y" {
		t.Error("YSuffix(1) should be 'y'")
	}
	if YSuffix(2) != "ies" {
		t.Error("YSuffix(2) should be 'ies'")
	}
}

func TestSortedKeys(t *testing.T) {
	m := map[string]bool{"banana": true, "apple": true, "cherry": true}
	got := SortedKeys(m)
	want := []string{"apple", "banana", "cherry"}
	if len(got) != len(want) {
		t.Fatalf("SortedKeys: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("SortedKeys[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	// Empty map
	empty := SortedKeys(map[string]int{})
	if len(empty) != 0 {
		t.Errorf("SortedKeys(empty) = %v, want []", empty)
	}
}

func TestSafeReadFile(t *testing.T) {
	dir := t.TempDir()

	regular := filepath.Join(dir, "regular.txt")
	if err := os.WriteFile(regular, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := SafeReadFile(dir, regular)
	if err != nil {
		t.Fatalf("SafeReadFile(regular): %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("SafeReadFile(regular) = %q, want %q", got, "hello")
	}

	if !IsRegularFile(regular) {
		t.Errorf("IsRegularFile(regular) = false, want true")
	}

	if runtime.GOOS != "windows" {
		secret := filepath.Join(dir, "secret.txt")
		if err := os.WriteFile(secret, []byte("PRIVATE"), 0o644); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(dir, "link.txt")
		if err := os.Symlink(secret, link); err != nil {
			t.Fatal(err)
		}
		if _, err := SafeReadFile(dir, link); !errors.Is(err, ErrUnsafeFile) {
			t.Errorf("SafeReadFile(symlink) error = %v, want ErrUnsafeFile", err)
		}
		if IsRegularFile(link) {
			t.Errorf("IsRegularFile(symlink) = true, want false")
		}
	}

	missing := filepath.Join(dir, "missing.txt")
	if _, err := SafeReadFile(dir, missing); err == nil {
		t.Errorf("SafeReadFile(missing) error = nil, want non-nil")
	}
	if IsRegularFile(missing) {
		t.Errorf("IsRegularFile(missing) = true, want false")
	}

	if _, err := SafeReadFile(dir, dir); !errors.Is(err, ErrUnsafeFile) {
		t.Errorf("SafeReadFile(dir) error = %v, want ErrUnsafeFile", err)
	}
}

// TestSafeReadFile_SymlinkedDirectory covers issue #88: a regular file inside
// a symlinked directory passes a leaf-only Lstat check, but its resolved path
// escapes the skill root and must be refused.
func TestSafeReadFile_SymlinkedDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks require admin on Windows")
	}

	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("OUT-OF-TREE"), 0o644); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "references")); err != nil {
		t.Fatal(err)
	}

	// The leaf is a regular file, so only the resolved-path containment
	// check can catch the escape.
	leaked := filepath.Join(root, "references", "secret.md")
	if _, err := SafeReadFile(root, leaked); !errors.Is(err, ErrUnsafeFile) {
		t.Errorf("SafeReadFile(file in symlinked dir) error = %v, want ErrUnsafeFile", err)
	}

	// A symlinked subdirectory that stays inside the root is fine.
	if err := os.MkdirAll(filepath.Join(root, "assets", "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "real", "ok.md"), []byte("fine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "assets", "real"), filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := SafeReadFile(root, filepath.Join(root, "alias", "ok.md")); err != nil {
		t.Errorf("SafeReadFile(file in in-tree symlinked dir) error = %v, want nil", err)
	}
}

func TestResolvesWithin(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "references")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	inside, err := ResolvesWithin(root, sub)
	if err != nil || !inside {
		t.Errorf("ResolvesWithin(root, root/references) = %v, %v; want true, nil", inside, err)
	}
	inside, err = ResolvesWithin(root, root)
	if err != nil || !inside {
		t.Errorf("ResolvesWithin(root, root) = %v, %v; want true, nil", inside, err)
	}
	outside := t.TempDir()
	inside, err = ResolvesWithin(root, outside)
	if err != nil || inside {
		t.Errorf("ResolvesWithin(root, outside) = %v, %v; want false, nil", inside, err)
	}
}
