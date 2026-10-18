//go:build windows

package skillcheck

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/agent-ecosystem/skill-validator/internal/testutil"
)

// Issue #101: Go reports a directory junction as an irregular file, neither
// a directory nor a symlink, so discovery dropped every junctioned skill
// without a word. Junctions need no privilege, so these run on any Windows
// host.
func TestDiscoverSkills_Junctions(t *testing.T) {
	t.Run("junctioned catalog is discovered", func(t *testing.T) {
		// The reported layout: a real source tree elsewhere and a skills
		// directory made of junctions into it, beside one ordinary skill.
		source := t.TempDir()
		dir := t.TempDir()
		var want []string
		for _, name := range []string{"alpha", "beta", "gamma"} {
			writeSkill(t, filepath.Join(source, name), "---\nname: "+name+"\n---\n")
			testutil.Junction(t, filepath.Join(source, name), filepath.Join(dir, name))
			want = append(want, filepath.Join(dir, name))
		}
		writeSkill(t, filepath.Join(dir, "plain"), "---\nname: plain\n---\n")
		want = append(want, filepath.Join(dir, "plain"))

		d := DiscoverSkills(dir, DefaultMaxDepth)
		if !reflect.DeepEqual(d.Dirs, want) {
			t.Errorf("dirs = %v, want %v", d.Dirs, want)
		}
	})

	t.Run("junction to a grouping directory is searched", func(t *testing.T) {
		source := t.TempDir()
		writeSkill(t, filepath.Join(source, "team", "skill"), "---\nname: s\n---\n")
		dir := t.TempDir()
		testutil.Junction(t, source, filepath.Join(dir, "linked"))

		d := DiscoverSkills(dir, DefaultMaxDepth)
		if want := []string{filepath.Join(dir, "linked", "team", "skill")}; !reflect.DeepEqual(d.Dirs, want) {
			t.Errorf("dirs = %v, want %v", d.Dirs, want)
		}
	})

	t.Run("alias of a skill is reported once", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, filepath.Join(dir, "real"), "---\nname: s\n---\n")
		testutil.Junction(t, filepath.Join(dir, "real"), filepath.Join(dir, "alias"))

		d := DiscoverSkills(dir, DefaultMaxDepth)
		if len(d.Dirs) != 1 {
			t.Errorf("dirs = %v, want exactly one", d.Dirs)
		}
	})

	t.Run("cycle to root", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, filepath.Join(dir, "team", "skill"), "---\nname: s\n---\n")
		testutil.Junction(t, dir, filepath.Join(dir, "loop"))

		d := DiscoverSkills(dir, DefaultMaxDepth)
		if want := []string{filepath.Join(dir, "team", "skill")}; !reflect.DeepEqual(d.Dirs, want) {
			t.Errorf("dirs = %v, want %v", d.Dirs, want)
		}
		if d.Truncated {
			t.Error("a cycle must not consume the directory budget")
		}
	})

	t.Run("dangling junction is ignored", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, filepath.Join(dir, "skill"), "---\nname: s\n---\n")
		gone := filepath.Join(t.TempDir(), "gone")
		if err := os.Mkdir(gone, 0o755); err != nil {
			t.Fatal(err)
		}
		testutil.Junction(t, gone, filepath.Join(dir, "dangling"))
		if err := os.Remove(gone); err != nil {
			t.Fatal(err)
		}

		d := DiscoverSkills(dir, DefaultMaxDepth)
		if want := []string{filepath.Join(dir, "skill")}; !reflect.DeepEqual(d.Dirs, want) {
			t.Errorf("dirs = %v, want %v", d.Dirs, want)
		}
	})
}
