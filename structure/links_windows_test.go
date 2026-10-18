//go:build windows

package structure

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agent-ecosystem/skill-validator/internal/testutil"
	"github.com/agent-ecosystem/skill-validator/types"
)

func TestCheckInternalLinks_Junctions(t *testing.T) {
	// Issue #86 for junctions: a link that looks internal may cross a
	// junction that leads outside the skill.
	t.Run("junction escaping skill directory is rejected", func(t *testing.T) {
		outside := t.TempDir()
		if err := os.WriteFile(filepath.Join(outside, "secret.md"), []byte("OUT-OF-TREE"), 0o644); err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		testutil.Junction(t, outside, filepath.Join(dir, "references"))

		body := "See [secret](references/secret.md)."
		results := CheckInternalLinks(dir, body)
		requireResult(t, results, types.Error, "internal link escapes skill directory: references/secret.md (resolves outside the skill package)")
	})

	// Issue #101: a skill that is itself reached through a junction has
	// every internal link resolve through that junction, so they are inside.
	t.Run("links inside a junctioned skill pass", func(t *testing.T) {
		source := t.TempDir()
		writeFile(t, source, "references/guide.md", "content")
		dir := filepath.Join(t.TempDir(), "skill")
		testutil.Junction(t, source, dir)

		body := "See [guide](references/guide.md)."
		results := CheckInternalLinks(dir, body)
		requireResult(t, results, types.Pass, "internal link: references/guide.md (exists)")
	})
}
