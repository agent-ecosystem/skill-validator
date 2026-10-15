// Package skillcheck provides skill detection and reference analysis
// operations. Type definitions (Level, Result, Report, etc.) live in
// the types package.
package skillcheck

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agent-ecosystem/skill-validator/contamination"
	"github.com/agent-ecosystem/skill-validator/content"
	"github.com/agent-ecosystem/skill-validator/types"
	"github.com/agent-ecosystem/skill-validator/util"
)

// Discovery bounds. The agentskills.io client implementation guide
// recommends that scanners "set reasonable bounds (e.g., max depth of 4-6
// levels, max 2000 directories) to prevent runaway scanning in large
// directory trees".
const (
	// DefaultMaxDepth is how many directory levels below the given path
	// DetectSkills searches for SKILL.md files. Depth 1 is the immediate
	// subdirectories.
	DefaultMaxDepth = 6

	// MaxDiscoveryDirs is the most directories a discovery scan lists
	// before it stops. A scan that stops early reports Truncated.
	MaxDiscoveryDirs = 2000
)

// skipDirs are directory names that never hold skills and can be large
// enough to use up the discovery budget. Hidden directories (".git",
// ".venv", ...) are skipped separately by name prefix.
var skipDirs = map[string]bool{
	"node_modules": true,
	"__pycache__":  true,
}

// Discovery is the result of DiscoverSkills.
type Discovery struct {
	Mode types.SkillMode
	// Dirs holds the skill directories in sorted order. For SingleSkill it
	// is the given path; for NoSkill it is nil.
	Dirs []string
	// Depths maps each entry of Dirs to how many directory levels below
	// the given path it was found at: 0 for a single skill, 1 for an
	// immediate subdirectory. Agents differ in how deep they look, so a
	// skill found at depth 2 may be invisible to some of them.
	Depths map[string]int
	// Truncated reports that the scan stopped at MaxDiscoveryDirs before it
	// had examined the whole tree, so skills below that point were missed.
	Truncated bool
}

// DetectSkills determines whether dir is a single skill, a multi-skill
// parent, or contains no skills, searching DefaultMaxDepth levels deep.
// See DiscoverSkills.
func DetectSkills(dir string) (types.SkillMode, []string) {
	d := DiscoverSkills(dir, DefaultMaxDepth)
	return d.Mode, d.Dirs
}

// DiscoverSkills determines whether root is a single skill, a multi-skill
// parent, or contains no skills.
//
// If root itself contains SKILL.md it is a single skill and nothing below
// it is examined. Otherwise the subdirectories are searched breadth-first,
// up to maxDepth levels below root (1 means the immediate subdirectories
// only), and every directory holding a SKILL.md is a skill. A skill's own
// subtree is not searched further, so a SKILL.md nested inside a skill's
// references or examples does not count as a second skill. Hidden
// directories and dependency directories such as node_modules are
// skipped. Only the given path is scanned: the user's home directory and
// the .agents/skills conventions are an agent's concern, not a
// validator's.
//
// Symlinks are followed, so a skills directory assembled from links to
// skill folders elsewhere works. Each real directory is examined once,
// which makes symlink cycles and aliases harmless and keeps a skill from
// being reported under two paths. The scan lists at most MaxDiscoveryDirs
// directories and reports Truncated if it had to stop. A skill is found
// as soon as its parent directory is listed, and the search is
// breadth-first, so shallow skills are found before a large or linked
// subtree can exhaust that budget.
func DiscoverSkills(root string, maxDepth int) Discovery {
	return discoverSkills(root, maxDepth, MaxDiscoveryDirs)
}

func discoverSkills(root string, maxDepth, maxDirs int) Discovery {
	if hasSkillFile(root) {
		return Discovery{Mode: types.SingleSkill, Dirs: []string{root}, Depths: map[string]int{root: 0}}
	}
	if maxDepth < 1 {
		return Discovery{Mode: types.NoSkill}
	}

	// A queued directory is one that will be listed. Its children are
	// classified as they are listed: a child with SKILL.md is a skill and
	// is not queued; any other child is queued if it is shallow enough to
	// be listed in turn. So a skill is found as soon as its parent is
	// listed, and the budget is spent on listings, not on skills.
	type dirItem struct {
		path  string // path as reached from root, reported to the caller
		real  string // absolute, symlink-resolved path, used to visit each directory once
		depth int
	}

	// Identity keys must be absolute: a relative root would otherwise give
	// relative keys to ordinary children while a link to an absolute target
	// resolves to an absolute key, and the same directory would be visited
	// twice. Caller-facing paths are left as given.
	visited := make(map[string]bool)
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Discovery{Mode: types.NoSkill}
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return Discovery{Mode: types.NoSkill}
	}
	visited[realRoot] = true

	queue := []dirItem{{path: root, real: realRoot, depth: 0}}
	var skillDirs []string
	depths := make(map[string]int)
	listed := 0
	truncated := false

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if listed >= maxDirs {
			truncated = true
			break
		}
		listed++

		entries, err := os.ReadDir(cur.path)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, ".") || skipDirs[name] {
				continue
			}
			sub := filepath.Join(cur.path, name)
			var real string
			switch {
			case entry.IsDir():
				real = filepath.Join(cur.real, name)
			case entry.Type()&os.ModeSymlink != 0:
				// Follow the link, but only into a directory that exists.
				// Resolving through cur.real keeps the key absolute.
				resolved, err := filepath.EvalSymlinks(filepath.Join(cur.real, name))
				if err != nil {
					continue
				}
				if info, err := os.Stat(resolved); err != nil || !info.IsDir() {
					continue
				}
				real = resolved
			default:
				continue
			}
			if visited[real] {
				continue
			}
			visited[real] = true

			if hasSkillFile(sub) {
				skillDirs = append(skillDirs, sub)
				depths[sub] = cur.depth + 1
				continue
			}
			if cur.depth+1 < maxDepth {
				queue = append(queue, dirItem{path: sub, real: real, depth: cur.depth + 1})
			}
		}
	}

	if len(skillDirs) == 0 {
		return Discovery{Mode: types.NoSkill, Truncated: truncated}
	}
	sort.Strings(skillDirs)
	return Discovery{Mode: types.MultiSkill, Dirs: skillDirs, Depths: depths, Truncated: truncated}
}

// hasSkillFile reports whether dir contains a SKILL.md (following symlinks).
func hasSkillFile(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "SKILL.md"))
	return err == nil
}

// ReadSkillRaw reads the raw SKILL.md content from a directory without parsing
// frontmatter. This is used as a fallback for content/contamination analysis when
// frontmatter parsing fails.
func ReadSkillRaw(dir string) string {
	data, err := util.SafeReadFile(dir, filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return ""
	}
	return string(data)
}

// ReadReferencesMarkdownFiles reads all .md files from <dir>/references/ and returns
// a map from filename to content. Returns nil if no references dir or no .md files
// are found.
func ReadReferencesMarkdownFiles(dir string) map[string]string {
	refsDir := filepath.Join(dir, "references")
	entries, err := os.ReadDir(refsDir)
	if err != nil {
		return nil
	}

	files := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		data, err := util.SafeReadFile(dir, filepath.Join(refsDir, entry.Name()))
		if err != nil {
			continue
		}
		files[entry.Name()] = string(data)
	}

	if len(files) == 0 {
		return nil
	}
	return files
}

// AnalyzeReferences runs content and contamination analysis on reference markdown
// files. It populates the aggregate ReferencesContentReport, ReferencesContaminationReport,
// and per-file ReferenceReports on the given report.
func AnalyzeReferences(dir string, rpt *types.Report) {
	files := ReadReferencesMarkdownFiles(dir)
	if files == nil {
		return
	}

	// Sort filenames for deterministic ordering
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	// Per-file analysis
	var parts []string
	for _, name := range names {
		fileContent := files[name]
		parts = append(parts, fileContent)

		fr := types.ReferenceFileReport{File: name}
		fr.ContentReport = content.Analyze(fileContent)
		skillName := util.SkillNameFromDir(dir)
		fr.ContaminationReport = contamination.Analyze(skillName, fileContent, fr.ContentReport.CodeLanguages)
		rpt.ReferenceReports = append(rpt.ReferenceReports, fr)
	}

	// Aggregate analysis on concatenated content
	concatenated := strings.Join(parts, "\n")
	rpt.ReferencesContentReport = content.Analyze(concatenated)
	skillName := util.SkillNameFromDir(dir)
	rpt.ReferencesContaminationReport = contamination.Analyze(skillName, concatenated, rpt.ReferencesContentReport.CodeLanguages)
}
