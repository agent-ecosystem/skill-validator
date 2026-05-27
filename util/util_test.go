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
	got, err := SafeReadFile(regular)
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
		if _, err := SafeReadFile(link); !errors.Is(err, ErrUnsafeFile) {
			t.Errorf("SafeReadFile(symlink) error = %v, want ErrUnsafeFile", err)
		}
		if IsRegularFile(link) {
			t.Errorf("IsRegularFile(symlink) = true, want false")
		}
	}

	missing := filepath.Join(dir, "missing.txt")
	if _, err := SafeReadFile(missing); err == nil {
		t.Errorf("SafeReadFile(missing) error = nil, want non-nil")
	}
	if IsRegularFile(missing) {
		t.Errorf("IsRegularFile(missing) = true, want false")
	}

	if _, err := SafeReadFile(dir); !errors.Is(err, ErrUnsafeFile) {
		t.Errorf("SafeReadFile(dir) error = %v, want ErrUnsafeFile", err)
	}
}
