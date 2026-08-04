package structure

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agent-ecosystem/skill-validator/types"
	"github.com/agent-ecosystem/skill-validator/util"
)

// orderedRecognizedDirs lists the recognized subdirectories in a stable order
// for deterministic output. These match the keys in recognizedDirs (checks.go).
var orderedRecognizedDirs = []string{"assets", "references", "scripts"}

// queueItem represents a text body to scan during the BFS reachability walk.
type queueItem struct {
	text   string
	source string // file that provided this text ("SKILL.md" for the seed)
}

// CheckOrphanFiles walks scripts/, references/, and assets/ to find files
// that are never referenced (directly or transitively) from SKILL.md.
// Directories listed in opts.AllowDirs are skipped with an informational note,
// since the validator cannot know their expected reference patterns.
func CheckOrphanFiles(dir, body string, opts Options) []types.Result {
	ctx := types.ResultContext{Category: "Structure"}
	var results []types.Result

	// Emit informational notes for allowed directories that exist on disk.
	// These are skipped for orphan detection because the validator cannot
	// know their expected reference patterns.
	for _, ad := range opts.AllowDirs {
		if recognizedDirs[ad] {
			continue // already covered by normal orphan detection
		}
		if _, err := os.Stat(filepath.Join(dir, ad)); err == nil {
			results = append(results, ctx.Infof(
				"%s/ skipped for orphan detection (allowed via --allow-dirs)", ad))
		}
	}

	// Inventory: collect all files in recognized directories.
	inventory := inventoryFiles(dir)
	if len(inventory) == 0 {
		return results
	}

	// Collect root-level text files (excluding SKILL.md) that can serve as
	// intermediaries in the reference chain. These aren't in the inventory
	// (we don't check whether they're orphaned), but they can bridge
	// SKILL.md to files in recognized directories (e.g., FORMS.md, package.json).
	rootFiles := rootTextFiles(dir)

	// BFS reachability from SKILL.md body.
	reached := make(map[string]bool)          // relPath → true
	reachedFrom := make(map[string]string)    // relPath → parent that first referenced it ("SKILL.md" for direct)
	missingExtension := make(map[string]bool) // relPath → true if matched only without file extension
	scannedRootFiles := make(map[string]bool)
	scannedInitFiles := make(map[string]bool)

	// Seed the queue with the SKILL.md body.
	queue := []queueItem{{text: body, source: "SKILL.md"}}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		// Determine the directory of the source file so we can resolve
		// relative paths. For SKILL.md (the seed), the base is the root.
		sourceDir := ""
		if item.source != "SKILL.md" {
			sourceDir = filepath.Dir(item.source)
		}

		// Check if the current text references any root-level files we
		// haven't scanned yet. If so, read and enqueue them as intermediaries.
		// Use case-insensitive matching since skill authors commonly use
		// different casing (e.g., "FORMS.md" in text, "forms.md" on disk).
		lowerText := strings.ToLower(item.text)
		for _, rf := range rootFiles {
			if scannedRootFiles[rf] {
				continue
			}
			if strings.Contains(lowerText, strings.ToLower(rf)) {
				scannedRootFiles[rf] = true
				data, err := util.SafeReadFile(dir, filepath.Join(dir, rf))
				if err == nil {
					queue = append(queue, queueItem{text: string(data), source: rf})
				}
			}
		}

		isPython := strings.HasSuffix(item.source, ".py")

		for _, relPath := range inventory {
			if reached[relPath] {
				continue
			}
			if containsReference(item.text, sourceDir, relPath) {
				markReached(relPath, item.source, dir, &queue, reached, reachedFrom)
			} else if isPython && pythonImportReaches(item.text, item.source, relPath) {
				// Python import resolution takes priority over the extensionless
				// fallback so that normal import statements (e.g., "from helpers
				// import merge") don't trigger a "missing extension" warning.
				markReached(relPath, item.source, dir, &queue, reached, reachedFrom)
			} else if containsReferenceWithoutExtension(item.text, sourceDir, relPath) {
				markReached(relPath, item.source, dir, &queue, reached, reachedFrom)
				missingExtension[relPath] = true
			}
		}

		// For Python files, check if any imports resolve to package directories
		// (i.e., directories with __init__.py). The __init__.py files are excluded
		// from inventory so they don't get orphan warnings, but they can act as
		// bridges: e.g., pack.py does "from validators import X" which hits
		// validators/__init__.py, which re-exports from .base, .docx, etc.
		if isPython {
			for _, initPath := range pythonPackageInits(item.text, item.source, dir) {
				if scannedInitFiles[initPath] {
					continue
				}
				scannedInitFiles[initPath] = true
				data, err := util.SafeReadFile(dir, filepath.Join(dir, initPath))
				if err == nil {
					queue = append(queue, queueItem{text: string(data), source: initPath})
				}
			}
		}
	}

	// Build results per directory.
	for _, d := range orderedRecognizedDirs {
		dirFiles := filesInDir(inventory, d)
		if len(dirFiles) == 0 {
			continue
		}

		hasOrphans := false
		for _, relPath := range dirFiles {
			if !reached[relPath] {
				hasOrphans = true
				results = append(results, ctx.WarnFile(relPath,
					fmt.Sprintf("potentially unreferenced file: %s — agents may not discover this file without an explicit reference in SKILL.md or a referenced file", relPath)))
			} else if missingExtension[relPath] {
				ext := filepath.Ext(relPath)
				noExt := strings.TrimSuffix(relPath, ext)
				results = append(results, ctx.WarnFile(relPath,
					fmt.Sprintf("file %s is referenced without its extension (as %s in %s) — include the %s extension so agents can reliably locate the file", relPath, noExt, reachedFrom[relPath], ext)))
			}
		}

		if !hasOrphans {
			results = append(results, ctx.Passf("all files in %s/ are referenced", d))
		}
	}

	return results
}

// rootTextFiles returns the names of text files in the skill root directory,
// excluding SKILL.md. These files aren't tracked as inventory (we don't warn
// about them being orphaned), but they participate in the BFS as intermediaries
// that can bridge SKILL.md to files in recognized directories.
func rootTextFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !entry.Type().IsRegular() {
			continue
		}
		name := entry.Name()
		if strings.EqualFold(name, "SKILL.md") {
			continue
		}
		if isTextFile(name) {
			files = append(files, name)
		}
	}
	return files
}

// inventoryFiles collects relative paths for all files under recognized directories.
func inventoryFiles(dir string) []string {
	var files []string
	for _, d := range orderedRecognizedDirs {
		subdir := filepath.Join(dir, d)
		err := filepath.WalkDir(subdir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return nil // skip inaccessible paths
			}
			if entry.IsDir() {
				return nil
			}
			if !entry.Type().IsRegular() {
				return nil
			}
			// Skip __init__.py files — these are Python package markers that
			// are never referenced by name. Warning about them is pure noise:
			// if siblings are reached they're implicitly needed, and if the
			// whole directory is orphaned the other files will be flagged.
			if entry.Name() == "__init__.py" {
				return nil
			}
			rel, _ := filepath.Rel(dir, path)
			files = append(files, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			continue
		}
	}
	return files
}

// filesInDir returns inventory entries that start with the given directory prefix.
func filesInDir(inventory []string, dir string) []string {
	prefix := dir + "/"
	var out []string
	for _, f := range inventory {
		if strings.HasPrefix(f, prefix) {
			out = append(out, f)
		}
	}
	return out
}

// containsReference checks whether text references relPath, either by its full
// root-relative path or by a path relative to sourceDir. For example, a file at
// references/guide.md might reference images/diagram.png, which should match the
// inventory entry references/images/diagram.png.
func containsReference(text, sourceDir, relPath string) bool {
	return matchesReference(text, sourceDir, relPath, false)
}

// containsReferenceWithoutExtension is like containsReference but strips the
// file extension before matching. This catches cases where skill authors
// reference scripts without the extension (e.g., "scripts/check_fillable_fields"
// instead of "scripts/check_fillable_fields.py"). The sourceDir-relative
// fallback is skipped when it would reduce to a bare filename: without an
// extension or a directory separator, that form is indistinguishable from an
// ordinary word in prose (a same-directory sibling of lint.yml would
// otherwise match every "lint" in the text — see issue #85).
func containsReferenceWithoutExtension(text, sourceDir, relPath string) bool {
	ext := filepath.Ext(relPath)
	if ext == "" {
		return false
	}
	return matchesReference(text, sourceDir, strings.TrimSuffix(relPath, ext), true)
}

// matchesReference reports whether text mentions relPath by its root-relative
// form or by its form relative to sourceDir. When relRequiresDir is set, the
// sourceDir-relative fallback only applies if that form still contains a
// directory separator.
func matchesReference(text, sourceDir, relPath string, relRequiresDir bool) bool {
	if containsPathToken(text, relPath) {
		return true
	}
	if sourceDir != "" {
		rel, err := filepath.Rel(sourceDir, relPath)
		if err == nil && !strings.HasPrefix(rel, "..") {
			relSlash := filepath.ToSlash(rel)
			if (!relRequiresDir || strings.Contains(relSlash, "/")) && containsPathToken(text, relSlash) {
				return true
			}
		}
	}
	return false
}

// containsPathToken reports whether token occurs in text with path-word
// boundaries on both sides, so references/lint does not match inside
// references/linting and lint.yml does not match inside eslint.yml.
func containsPathToken(text, token string) bool {
	for start := 0; ; start++ {
		i := strings.Index(text[start:], token)
		if i < 0 {
			return false
		}
		start += i
		end := start + len(token)
		beforeOK := start == 0 || !isPathWordByte(text[start-1])
		afterOK := end == len(text) || !isPathWordByte(text[end])
		if beforeOK && afterOK {
			return true
		}
	}
}

func isPathWordByte(b byte) bool {
	return b == '_' || b == '-' ||
		('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || ('0' <= b && b <= '9')
}

// markReached marks a file as reached, reads it if it's a text file, and
// enqueues its content for further BFS scanning.
func markReached(relPath, source, dir string, queue *[]queueItem, reached map[string]bool, reachedFrom map[string]string) {
	reached[relPath] = true
	reachedFrom[relPath] = source

	if isTextFile(relPath) {
		data, err := util.SafeReadFile(dir, filepath.Join(dir, relPath))
		if err == nil {
			*queue = append(*queue, queueItem{text: string(data), source: relPath})
		}
	}
}

// pythonImportRe matches Python import statements:
//   - "from module import ..."
//   - "from .module import ..."
//   - "from ..module import ..."
//   - "import module"
var pythonImportRe = regexp.MustCompile(`(?m)^\s*(?:from\s+(\.{0,2}[\w.]+)\s+import|import\s+([\w.]+))`)

// pythonImportReaches checks whether a Python source file's import statements
// resolve to the given inventory path. Module paths like "helpers.merge_runs"
// are converted to file paths ("helpers/merge_runs.py") and resolved relative
// to the importing file's directory.
func pythonImportReaches(text, source, relPath string) bool {
	if !strings.HasSuffix(relPath, ".py") {
		return false
	}
	sourceDir := filepath.Dir(source)

	for _, match := range pythonImportRe.FindAllStringSubmatch(text, -1) {
		// match[1] is the "from X import" module, match[2] is the "import X" module
		module := match[1]
		if module == "" {
			module = match[2]
		}

		// Handle relative imports: the first dot means "current package"
		// (same directory as the importing file). Each additional dot goes
		// one level up (.. = parent package, ... = grandparent, etc.).
		resolveDir := sourceDir
		if strings.HasPrefix(module, ".") {
			module = module[1:] // first dot: current package (no directory change)
			for strings.HasPrefix(module, ".") {
				module = module[1:]
				resolveDir = filepath.Dir(resolveDir)
			}
		}
		if module == "" {
			continue
		}

		// Convert dotted module path to file path: helpers.merge_runs → helpers/merge_runs
		modulePath := strings.ReplaceAll(module, ".", "/")

		// Try resolving as a .py file relative to the source directory.
		candidate := filepath.ToSlash(filepath.Join(resolveDir, modulePath+".py"))
		if candidate == relPath {
			return true
		}
	}
	return false
}

// pythonPackageInits returns relative paths to __init__.py files for any
// Python imports in text that resolve to package directories rather than .py
// files. For example, "from validators import X" in scripts/office/pack.py
// resolves to scripts/office/validators/__init__.py if that file exists on disk.
func pythonPackageInits(text, source, dir string) []string {
	sourceDir := filepath.Dir(source)
	var inits []string

	for _, match := range pythonImportRe.FindAllStringSubmatch(text, -1) {
		module := match[1]
		if module == "" {
			module = match[2]
		}

		resolveDir := sourceDir
		if strings.HasPrefix(module, ".") {
			module = module[1:]
			for strings.HasPrefix(module, ".") {
				module = module[1:]
				resolveDir = filepath.Dir(resolveDir)
			}
		}
		if module == "" {
			continue
		}

		modulePath := strings.ReplaceAll(module, ".", "/")
		initPath := filepath.ToSlash(filepath.Join(resolveDir, modulePath, "__init__.py"))

		// Check if the __init__.py actually exists on disk.
		if _, err := os.Stat(filepath.Join(dir, initPath)); err == nil {
			inits = append(inits, initPath)
		}
	}
	return inits
}

// CheckFlatOrphanFiles checks root-level non-SKILL.md files for references in
// the SKILL.md body. Files not referenced are reported as potentially orphaned.
// This is a simpler check than CheckOrphanFiles since all files are at the root.
func CheckFlatOrphanFiles(dir, body string) []types.Result {
	ctx := types.ResultContext{Category: "Structure"}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var rootFiles []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		if !entry.Type().IsRegular() {
			continue
		}
		if strings.EqualFold(name, "SKILL.md") {
			continue
		}
		rootFiles = append(rootFiles, name)
	}

	if len(rootFiles) == 0 {
		return nil
	}

	var results []types.Result
	hasOrphans := false
	for _, name := range rootFiles {
		if !containsReference(body, "", name) &&
			!containsReferenceWithoutExtension(body, "", name) {
			hasOrphans = true
			results = append(results, ctx.WarnFile(name,
				fmt.Sprintf("potentially unreferenced file: %s — agents may not discover this file without an explicit reference in SKILL.md", name)))
		}
	}

	if !hasOrphans {
		results = append(results, ctx.Pass("all root-level files are referenced from SKILL.md"))
	}

	return results
}

// isTextFile checks whether the file extension indicates a scannable text file.
// Anything not in the binary extension list is assumed to be text.
func isTextFile(relPath string) bool {
	return !binaryExtensions[strings.ToLower(filepath.Ext(relPath))]
}
