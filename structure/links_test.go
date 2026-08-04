package structure

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/agent-ecosystem/skill-validator/types"
)

func TestCheckInternalLinks(t *testing.T) {
	t.Run("existing file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "references/guide.md", "content")
		body := "See [guide](references/guide.md)."
		results := CheckInternalLinks(dir, body)
		requireResult(t, results, types.Pass, "internal link: references/guide.md (exists)")
	})

	t.Run("missing file", func(t *testing.T) {
		dir := t.TempDir()
		body := "See [guide](references/missing.md)."
		results := CheckInternalLinks(dir, body)
		requireResult(t, results, types.Error, "broken internal link: references/missing.md (file not found)")
	})

	// Issue #86: a link that looks internal syntactically may be a symlink
	// whose target resolves outside the skill directory.
	t.Run("symlink escaping skill directory is rejected", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks require admin on Windows")
		}
		outside := t.TempDir()
		secret := filepath.Join(outside, "secret.md")
		if err := os.WriteFile(secret, []byte("OUT-OF-TREE"), 0o644); err != nil {
			t.Fatal(err)
		}

		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "references"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(secret, filepath.Join(dir, "references", "guide.md")); err != nil {
			t.Fatal(err)
		}

		body := "See [guide](references/guide.md)."
		results := CheckInternalLinks(dir, body)
		requireResult(t, results, types.Error, "internal link escapes skill directory: references/guide.md (resolves outside the skill package)")
	})

	t.Run("symlink resolving inside skill directory passes", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("symlinks require admin on Windows")
		}
		dir := t.TempDir()
		writeFile(t, dir, "references/real.md", "content")
		if err := os.Symlink(filepath.Join(dir, "references", "real.md"), filepath.Join(dir, "references", "alias.md")); err != nil {
			t.Fatal(err)
		}

		body := "See [alias](references/alias.md)."
		results := CheckInternalLinks(dir, body)
		requireResult(t, results, types.Pass, "internal link: references/alias.md (exists)")
	})

	t.Run("skips HTTP links", func(t *testing.T) {
		dir := t.TempDir()
		body := "[docs](https://example.com/docs)"
		results := CheckInternalLinks(dir, body)
		if len(results) != 0 {
			t.Errorf("expected 0 results for HTTP links, got %d", len(results))
		}
	})

	t.Run("skips mailto and anchors", func(t *testing.T) {
		dir := t.TempDir()
		body := "[email](mailto:user@example.com) and [section](#heading)"
		results := CheckInternalLinks(dir, body)
		if len(results) != 0 {
			t.Errorf("expected 0 results for mailto/anchor links, got %d", len(results))
		}
	})

	t.Run("skips template URLs", func(t *testing.T) {
		dir := t.TempDir()
		body := "[PR](https://github.com/{OWNER}/{REPO}/pull/{PR})"
		results := CheckInternalLinks(dir, body)
		if len(results) != 0 {
			t.Errorf("expected 0 results for template URLs, got %d", len(results))
		}
	})

	t.Run("file link with fragment identifier", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "references/guide.md", "# Heading\ncontent")
		body := "See [config](references/guide.md#heading)."
		results := CheckInternalLinks(dir, body)
		requireResult(t, results, types.Pass, "internal link: references/guide.md (exists)")
	})

	t.Run("path traversal is blocked", func(t *testing.T) {
		dir := t.TempDir()
		body := "See [escape](../../../../../../etc/passwd)."
		results := CheckInternalLinks(dir, body)
		requireResult(t, results, types.Error, "internal link escapes skill directory: ../../../../../../etc/passwd")
	})

	t.Run("no links returns nil", func(t *testing.T) {
		dir := t.TempDir()
		body := "No links here."
		results := CheckInternalLinks(dir, body)
		if results != nil {
			t.Errorf("expected nil for no links, got %v", results)
		}
	})

	t.Run("mixed internal and external only checks internal", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "references/guide.md", "content")
		body := "[guide](references/guide.md) and [site](https://example.com)"
		results := CheckInternalLinks(dir, body)
		if len(results) != 1 {
			t.Fatalf("expected 1 result (internal only), got %d", len(results))
		}
		requireResult(t, results, types.Pass, "internal link: references/guide.md (exists)")
	})

	t.Run("category is Structure", func(t *testing.T) {
		dir := t.TempDir()
		body := "See [guide](references/missing.md)."
		results := CheckInternalLinks(dir, body)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Category != "Structure" {
			t.Errorf("expected category %q, got %q", "Structure", results[0].Category)
		}
	})
}
