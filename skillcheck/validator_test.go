package skillcheck

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agent-ecosystem/skill-validator/internal/testutil"
	"github.com/agent-ecosystem/skill-validator/skill"
	"github.com/agent-ecosystem/skill-validator/types"
)

func TestLoadSkill(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "---\nname: test-skill\ndescription: A test\n---\n# Hello\n")

	s, err := skill.Load(dir)
	if err != nil {
		t.Fatalf("skill.Load error: %v", err)
	}
	if s.Frontmatter.Name != "test-skill" {
		t.Errorf("Name = %q, want test-skill", s.Frontmatter.Name)
	}
}

func TestLoadSkill_Missing(t *testing.T) {
	dir := t.TempDir()
	_, err := skill.Load(dir)
	if err == nil {
		t.Error("expected error for missing SKILL.md")
	}
}

func TestReadSkillRaw(t *testing.T) {
	dir := t.TempDir()
	content := "---\nname: raw\n---\n# Raw Skill\nSome content.\n"
	writeSkill(t, dir, content)

	got := ReadSkillRaw(dir)
	if got != content {
		t.Errorf("ReadSkillRaw = %q, want %q", got, content)
	}
}

func TestReadSkillRaw_Missing(t *testing.T) {
	dir := t.TempDir()
	got := ReadSkillRaw(dir)
	if got != "" {
		t.Errorf("ReadSkillRaw = %q, want empty", got)
	}
}

func TestReadReferencesMarkdownFiles(t *testing.T) {
	dir := t.TempDir()
	refsDir := filepath.Join(dir, "references")
	if err := os.MkdirAll(refsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refsDir, "guide.md"), []byte("# Guide"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refsDir, "notes.md"), []byte("# Notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refsDir, "data.json"), []byte(`{"skip": true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(refsDir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	files := ReadReferencesMarkdownFiles(dir)
	if files == nil {
		t.Fatal("expected non-nil map")
	}
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d", len(files))
	}
	if files["guide.md"] != "# Guide" {
		t.Errorf("guide.md content = %q", files["guide.md"])
	}
	if files["notes.md"] != "# Notes" {
		t.Errorf("notes.md content = %q", files["notes.md"])
	}
}

// Issue #88: when references/ itself is a symlink to a directory outside the
// skill tree, the regular files inside pass a leaf-only check. Their content
// must not be read and shipped to the LLM judge.
func TestReadReferencesMarkdownFiles_SkipsSymlinkedDirectory(t *testing.T) {
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "leak.md"), []byte("SHOULD-NOT-LEAK"), 0o644); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	testutil.Symlink(t, outside, filepath.Join(dir, "references"))

	files := ReadReferencesMarkdownFiles(dir)
	for name, content := range files {
		if strings.Contains(content, "SHOULD-NOT-LEAK") {
			t.Errorf("out-of-tree content leaked via %s", name)
		}
	}
}

func TestReadReferencesMarkdownFiles_SkipsSymlinks(t *testing.T) {
	dir := t.TempDir()
	refsDir := filepath.Join(dir, "references")
	if err := os.MkdirAll(refsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refsDir, "real.md"), []byte("# Real"), 0o644); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(dir, "out-of-tree-secret.md")
	if err := os.WriteFile(secret, []byte("SHOULD-NOT-LEAK"), 0o600); err != nil {
		t.Fatal(err)
	}
	testutil.Symlink(t, secret, filepath.Join(refsDir, "leaky.md"))

	files := ReadReferencesMarkdownFiles(dir)
	if _, ok := files["leaky.md"]; ok {
		t.Fatalf("expected symlinked reference to be skipped, got %q", files["leaky.md"])
	}
	if got := files["real.md"]; got != "# Real" {
		t.Errorf("real.md content = %q, want %q", got, "# Real")
	}
	for name, content := range files {
		if strings.Contains(content, "SHOULD-NOT-LEAK") {
			t.Errorf("symlink target leaked via %s", name)
		}
	}
}

func TestReadReferencesMarkdownFiles_NoDir(t *testing.T) {
	dir := t.TempDir()
	files := ReadReferencesMarkdownFiles(dir)
	if files != nil {
		t.Errorf("expected nil, got %v", files)
	}
}

func TestReadReferencesMarkdownFiles_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "references"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := ReadReferencesMarkdownFiles(dir)
	if files != nil {
		t.Errorf("expected nil for empty references dir, got %v", files)
	}
}

func TestReadReferencesMarkdownFiles_OnlyNonMd(t *testing.T) {
	dir := t.TempDir()
	refsDir := filepath.Join(dir, "references")
	if err := os.MkdirAll(refsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refsDir, "data.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	files := ReadReferencesMarkdownFiles(dir)
	if files != nil {
		t.Errorf("expected nil when only non-.md files, got %v", files)
	}
}

func TestAnalyzeReferences_WithFiles(t *testing.T) {
	dir := t.TempDir()
	refsDir := filepath.Join(dir, "references")
	if err := os.MkdirAll(refsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refsDir, "alpha.md"), []byte("# Alpha\nUse this tool."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refsDir, "beta.md"), []byte("# Beta\nAnother reference."), 0o644); err != nil {
		t.Fatal(err)
	}

	rpt := &types.Report{SkillDir: dir}
	AnalyzeReferences(dir, rpt)

	if rpt.ReferencesContentReport == nil {
		t.Fatal("expected aggregate ReferencesContentReport")
	}
	if rpt.ReferencesContaminationReport == nil {
		t.Fatal("expected aggregate ReferencesContaminationReport")
	}
	if len(rpt.ReferenceReports) != 2 {
		t.Fatalf("expected 2 per-file reports, got %d", len(rpt.ReferenceReports))
	}
	// Sorted alphabetically
	if rpt.ReferenceReports[0].File != "alpha.md" {
		t.Errorf("first file = %q, want alpha.md", rpt.ReferenceReports[0].File)
	}
	if rpt.ReferenceReports[1].File != "beta.md" {
		t.Errorf("second file = %q, want beta.md", rpt.ReferenceReports[1].File)
	}
	for _, fr := range rpt.ReferenceReports {
		if fr.ContentReport == nil {
			t.Errorf("%s: expected ContentReport", fr.File)
		}
		if fr.ContaminationReport == nil {
			t.Errorf("%s: expected ContaminationReport", fr.File)
		}
	}
}

func TestAnalyzeReferences_NoFiles(t *testing.T) {
	dir := t.TempDir()
	rpt := &types.Report{SkillDir: dir}
	AnalyzeReferences(dir, rpt)

	if rpt.ReferencesContentReport != nil {
		t.Error("expected nil ReferencesContentReport")
	}
	if rpt.ReferencesContaminationReport != nil {
		t.Error("expected nil ReferencesContaminationReport")
	}
	if len(rpt.ReferenceReports) != 0 {
		t.Error("expected no ReferenceReports")
	}
}

// writeSkill creates a SKILL.md file in the given directory.
func writeSkill(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectSkills(t *testing.T) {
	t.Run("single skill", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: test\n---\n")
		mode, dirs := DetectSkills(dir)
		if mode != types.SingleSkill {
			t.Errorf("expected SingleSkill, got %d", mode)
		}
		if len(dirs) != 1 || dirs[0] != dir {
			t.Errorf("expected [%s], got %v", dir, dirs)
		}
	})

	t.Run("multi skill", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, filepath.Join(dir, "alpha"), "---\nname: alpha\n---\n")
		writeSkill(t, filepath.Join(dir, "beta"), "---\nname: beta\n---\n")
		mode, dirs := DetectSkills(dir)
		if mode != types.MultiSkill {
			t.Errorf("expected MultiSkill, got %d", mode)
		}
		if len(dirs) != 2 {
			t.Fatalf("expected 2 dirs, got %d", len(dirs))
		}
		// os.ReadDir returns sorted entries
		if filepath.Base(dirs[0]) != "alpha" || filepath.Base(dirs[1]) != "beta" {
			t.Errorf("expected [alpha, beta], got [%s, %s]", filepath.Base(dirs[0]), filepath.Base(dirs[1]))
		}
	})

	t.Run("no skills", func(t *testing.T) {
		dir := t.TempDir()
		mode, dirs := DetectSkills(dir)
		if mode != types.NoSkill {
			t.Errorf("expected NoSkill, got %d", mode)
		}
		if dirs != nil {
			t.Errorf("expected nil dirs, got %v", dirs)
		}
	})

	t.Run("SKILL.md at root takes precedence", func(t *testing.T) {
		dir := t.TempDir()
		// Root has SKILL.md AND subdirs with SKILL.md
		writeSkill(t, dir, "---\nname: root\n---\n")
		writeSkill(t, filepath.Join(dir, "sub"), "---\nname: sub\n---\n")
		mode, dirs := DetectSkills(dir)
		if mode != types.SingleSkill {
			t.Errorf("expected SingleSkill (root precedence), got %d", mode)
		}
		if len(dirs) != 1 || dirs[0] != dir {
			t.Errorf("expected [%s], got %v", dir, dirs)
		}
	})

	t.Run("skips hidden dirs", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, filepath.Join(dir, ".hidden"), "---\nname: hidden\n---\n")
		writeSkill(t, filepath.Join(dir, "visible"), "---\nname: visible\n---\n")
		mode, dirs := DetectSkills(dir)
		if mode != types.MultiSkill {
			t.Errorf("expected MultiSkill, got %d", mode)
		}
		if len(dirs) != 1 {
			t.Fatalf("expected 1 dir (hidden skipped), got %d", len(dirs))
		}
		if filepath.Base(dirs[0]) != "visible" {
			t.Errorf("expected visible, got %s", filepath.Base(dirs[0]))
		}
	})

	t.Run("ignores subdirs without SKILL.md", func(t *testing.T) {
		dir := t.TempDir()
		// Create a subdir without SKILL.md
		if err := os.MkdirAll(filepath.Join(dir, "no-skill"), 0o755); err != nil {
			t.Fatal(err)
		}
		writeSkill(t, filepath.Join(dir, "has-skill"), "---\nname: has-skill\n---\n")
		mode, dirs := DetectSkills(dir)
		if mode != types.MultiSkill {
			t.Errorf("expected MultiSkill, got %d", mode)
		}
		if len(dirs) != 1 {
			t.Fatalf("expected 1 dir, got %d", len(dirs))
		}
		if filepath.Base(dirs[0]) != "has-skill" {
			t.Errorf("expected has-skill, got %s", filepath.Base(dirs[0]))
		}
	})

	t.Run("follows symlinks", func(t *testing.T) {
		dir := t.TempDir()
		// Create a real skill dir outside
		realDir := filepath.Join(dir, "real")
		writeSkill(t, realDir, "---\nname: real\n---\n")
		// Create a parent with a symlink
		parent := filepath.Join(dir, "parent")
		if err := os.MkdirAll(parent, 0o755); err != nil {
			t.Fatal(err)
		}
		testutil.Symlink(t, realDir, filepath.Join(parent, "linked"))
		mode, dirs := DetectSkills(parent)
		if mode != types.MultiSkill {
			t.Errorf("expected MultiSkill, got %d", mode)
		}
		if len(dirs) != 1 {
			t.Fatalf("expected 1 dir, got %d", len(dirs))
		}
	})
}

// Issue #77: skills grouped under intermediate directories (for example
// skills/<product>/<skill>/SKILL.md) are discovered up to a depth bound.
func TestDiscoverSkills_Nested(t *testing.T) {
	t.Run("two levels", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, filepath.Join(dir, "cuopt", "cuopt-developer"), "---\nname: a\n---\n")
		writeSkill(t, filepath.Join(dir, "Megatron-Bridge", "bump-base-image"), "---\nname: b\n---\n")
		writeSkill(t, filepath.Join(dir, "Megatron-Bridge", "update-golden-values"), "---\nname: c\n---\n")
		d := DiscoverSkills(dir, DefaultMaxDepth)
		if d.Mode != types.MultiSkill {
			t.Fatalf("expected MultiSkill, got %d", d.Mode)
		}
		want := []string{
			filepath.Join(dir, "Megatron-Bridge", "bump-base-image"),
			filepath.Join(dir, "Megatron-Bridge", "update-golden-values"),
			filepath.Join(dir, "cuopt", "cuopt-developer"),
		}
		if !reflect.DeepEqual(d.Dirs, want) {
			t.Errorf("dirs = %v, want %v", d.Dirs, want)
		}
		if d.Truncated {
			t.Error("unexpected truncation")
		}
	})

	t.Run("mixed depths", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, filepath.Join(dir, "top"), "---\nname: top\n---\n")
		writeSkill(t, filepath.Join(dir, "group", "deep"), "---\nname: deep\n---\n")
		d := DiscoverSkills(dir, DefaultMaxDepth)
		want := []string{filepath.Join(dir, "group", "deep"), filepath.Join(dir, "top")}
		if !reflect.DeepEqual(d.Dirs, want) {
			t.Errorf("dirs = %v, want %v", d.Dirs, want)
		}
		wantDepths := map[string]int{want[0]: 2, want[1]: 1}
		if !reflect.DeepEqual(d.Depths, wantDepths) {
			t.Errorf("depths = %v, want %v", d.Depths, wantDepths)
		}
	})

	t.Run("single skill has depth 0", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "---\nname: s\n---\n")
		d := DiscoverSkills(dir, DefaultMaxDepth)
		if want := map[string]int{dir: 0}; !reflect.DeepEqual(d.Depths, want) {
			t.Errorf("depths = %v, want %v", d.Depths, want)
		}
	})

	t.Run("depth bound", func(t *testing.T) {
		dir := t.TempDir()
		at := func(depth int) string {
			p := dir
			for i := 1; i <= depth; i++ {
				p = filepath.Join(p, fmt.Sprintf("l%d", i))
			}
			return p
		}
		writeSkill(t, at(DefaultMaxDepth), "---\nname: in\n---\n")
		writeSkill(t, filepath.Join(at(DefaultMaxDepth-1), "other", "x"), "---\nname: out\n---\n") // depth 7
		d := DiscoverSkills(dir, DefaultMaxDepth)
		if want := []string{at(DefaultMaxDepth)}; !reflect.DeepEqual(d.Dirs, want) {
			t.Errorf("dirs = %v, want %v", d.Dirs, want)
		}
		if d.Truncated {
			t.Error("a depth cutoff is not a truncation")
		}
		// maxDepth 1 is the pre-#77 behavior: immediate subdirectories only.
		if d := DiscoverSkills(dir, 1); d.Mode != types.NoSkill {
			t.Errorf("maxDepth 1: expected NoSkill, got %d %v", d.Mode, d.Dirs)
		}
		// maxDepth 0 looks at the given directory only.
		if d := DiscoverSkills(at(DefaultMaxDepth), 0); d.Mode != types.SingleSkill {
			t.Errorf("maxDepth 0 on a skill: expected SingleSkill, got %d", d.Mode)
		}
		if d := DiscoverSkills(dir, 0); d.Mode != types.NoSkill {
			t.Errorf("maxDepth 0 on a parent: expected NoSkill, got %d", d.Mode)
		}
	})

	t.Run("does not descend into a skill", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, filepath.Join(dir, "skill"), "---\nname: skill\n---\n")
		writeSkill(t, filepath.Join(dir, "skill", "examples", "nested"), "---\nname: nested\n---\n")
		d := DiscoverSkills(dir, DefaultMaxDepth)
		if want := []string{filepath.Join(dir, "skill")}; !reflect.DeepEqual(d.Dirs, want) {
			t.Errorf("dirs = %v, want %v", d.Dirs, want)
		}
	})

	t.Run("skips hidden dirs at any depth", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, filepath.Join(dir, "group", ".hidden", "skill"), "---\nname: hidden\n---\n")
		writeSkill(t, filepath.Join(dir, "group", "shown"), "---\nname: shown\n---\n")
		d := DiscoverSkills(dir, DefaultMaxDepth)
		if want := []string{filepath.Join(dir, "group", "shown")}; !reflect.DeepEqual(d.Dirs, want) {
			t.Errorf("dirs = %v, want %v", d.Dirs, want)
		}
	})

	t.Run("skips dependency dirs", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, filepath.Join(dir, "node_modules", "pkg", "skill"), "---\nname: nm\n---\n")
		writeSkill(t, filepath.Join(dir, "__pycache__", "skill"), "---\nname: pyc\n---\n")
		writeSkill(t, filepath.Join(dir, "web", "node_modules", "skill"), "---\nname: nested\n---\n")
		writeSkill(t, filepath.Join(dir, "web", "skill"), "---\nname: web\n---\n")
		d := DiscoverSkills(dir, DefaultMaxDepth)
		if want := []string{filepath.Join(dir, "web", "skill")}; !reflect.DeepEqual(d.Dirs, want) {
			t.Errorf("dirs = %v, want %v", d.Dirs, want)
		}
	})

	t.Run("directory budget", func(t *testing.T) {
		dir := t.TempDir()
		// 50 directories sort before the one that holds a skill at depth 2.
		// Listing the root finds the depth-1 skill at once; the budget of
		// 20 listings then runs out before the depth-2 skill's parent is
		// listed, and the scan reports the truncation.
		for i := 0; i < 50; i++ {
			if err := os.MkdirAll(filepath.Join(dir, fmt.Sprintf("a%02d", i), "sub"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		writeSkill(t, filepath.Join(dir, "shallow"), "---\nname: shallow\n---\n")
		writeSkill(t, filepath.Join(dir, "zgroup", "deep"), "---\nname: deep\n---\n")

		d := discoverSkills(dir, DefaultMaxDepth, 20)
		if want := []string{filepath.Join(dir, "shallow")}; !reflect.DeepEqual(d.Dirs, want) {
			t.Errorf("dirs = %v, want %v", d.Dirs, want)
		}
		if !d.Truncated {
			t.Error("expected Truncated when the budget runs out with directories left")
		}

		// With budget for the whole tree, nothing is truncated.
		d = discoverSkills(dir, DefaultMaxDepth, MaxDiscoveryDirs)
		if len(d.Dirs) != 2 || d.Truncated {
			t.Errorf("dirs = %v truncated = %v, want 2 dirs and no truncation", d.Dirs, d.Truncated)
		}
	})
}

func TestDiscoverSkills_Symlinks(t *testing.T) {
	t.Run("cycle to root", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, filepath.Join(dir, "team", "skill"), "---\nname: s\n---\n")
		testutil.Symlink(t, dir, filepath.Join(dir, "loop"))
		d := DiscoverSkills(dir, DefaultMaxDepth)
		if want := []string{filepath.Join(dir, "team", "skill")}; !reflect.DeepEqual(d.Dirs, want) {
			t.Errorf("dirs = %v, want %v", d.Dirs, want)
		}
		if d.Truncated {
			t.Error("a cycle must not consume the directory budget")
		}
	})

	t.Run("cycle to parent", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, filepath.Join(dir, "team", "skill"), "---\nname: s\n---\n")
		testutil.Symlink(t, "..", filepath.Join(dir, "team", "up"))
		d := DiscoverSkills(dir, DefaultMaxDepth)
		if want := []string{filepath.Join(dir, "team", "skill")}; !reflect.DeepEqual(d.Dirs, want) {
			t.Errorf("dirs = %v, want %v", d.Dirs, want)
		}
	})

	t.Run("alias of a skill is reported once", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, filepath.Join(dir, "real"), "---\nname: s\n---\n")
		testutil.Symlink(t, filepath.Join(dir, "real"), filepath.Join(dir, "alias"))
		d := DiscoverSkills(dir, DefaultMaxDepth)
		if len(d.Dirs) != 1 {
			t.Errorf("dirs = %v, want exactly one", d.Dirs)
		}
	})

	t.Run("alias is reported once from a relative root", func(t *testing.T) {
		// With a relative root, ordinary children resolve to relative paths
		// while a link to an absolute target resolves to an absolute one.
		// Identity keys must not depend on that, or the alias is counted as
		// a second skill. Reported paths stay relative, as given.
		base := t.TempDir()
		writeSkill(t, filepath.Join(base, "catalog", "real"), "---\nname: s\n---\n")
		testutil.Symlink(t, filepath.Join(base, "catalog", "real"), filepath.Join(base, "catalog", "alias"))
		t.Chdir(base)
		d := DiscoverSkills("catalog", DefaultMaxDepth)
		if want := []string{filepath.Join("catalog", "alias")}; !reflect.DeepEqual(d.Dirs, want) {
			t.Errorf("dirs = %v, want %v", d.Dirs, want)
		}
	})

	t.Run("follows link to a skill outside root", func(t *testing.T) {
		// A skills directory assembled from links to skill folders kept
		// elsewhere is a supported layout.
		outside := t.TempDir()
		writeSkill(t, filepath.Join(outside, "shared"), "---\nname: shared\n---\n")
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "team"), 0o755); err != nil {
			t.Fatal(err)
		}
		testutil.Symlink(t, filepath.Join(outside, "shared"), filepath.Join(dir, "team", "shared"))
		d := DiscoverSkills(dir, DefaultMaxDepth)
		if want := []string{filepath.Join(dir, "team", "shared")}; !reflect.DeepEqual(d.Dirs, want) {
			t.Errorf("dirs = %v, want %v", d.Dirs, want)
		}
	})

	t.Run("broken link and link to file are ignored", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, filepath.Join(dir, "skill"), "---\nname: s\n---\n")
		testutil.Symlink(t, filepath.Join(dir, "missing"), filepath.Join(dir, "dangling"))
		testutil.Symlink(t, filepath.Join(dir, "skill", "SKILL.md"), filepath.Join(dir, "filelink"))
		d := DiscoverSkills(dir, DefaultMaxDepth)
		if want := []string{filepath.Join(dir, "skill")}; !reflect.DeepEqual(d.Dirs, want) {
			t.Errorf("dirs = %v, want %v", d.Dirs, want)
		}
	})

	t.Run("linked large tree cannot starve a sibling skill", func(t *testing.T) {
		big := t.TempDir()
		for i := 0; i < 30; i++ {
			if err := os.MkdirAll(filepath.Join(big, fmt.Sprintf("d%02d", i), "x", "y"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		dir := t.TempDir()
		// "aaa-link" sorts before "team", so a depth-first scan would spend
		// its budget inside the linked tree and report no skills.
		testutil.Symlink(t, big, filepath.Join(dir, "aaa-link"))
		writeSkill(t, filepath.Join(dir, "team", "skill"), "---\nname: s\n---\n")
		d := discoverSkills(dir, DefaultMaxDepth, 40)
		if want := []string{filepath.Join(dir, "team", "skill")}; !reflect.DeepEqual(d.Dirs, want) {
			t.Errorf("dirs = %v, want %v", d.Dirs, want)
		}
		if !d.Truncated {
			t.Error("expected Truncated")
		}
	})
}
