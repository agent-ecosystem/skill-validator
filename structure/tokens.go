package structure

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tiktoken-go/tokenizer"

	"github.com/agent-ecosystem/skill-validator/types"
	"github.com/agent-ecosystem/skill-validator/util"
)

const maxTokenizedFileBytes = 8 * 1024 * 1024

func readFileWithCap(root, path string) ([]byte, error) {
	data, err := util.SafeReadFile(root, path)
	if err != nil {
		return nil, err
	}
	if len(data) > maxTokenizedFileBytes {
		data = data[:maxTokenizedFileBytes]
	}
	return data, nil
}

const (
	// refFileSoftLimit is the per-file token warning threshold for reference files.
	refFileSoftLimit = 10_000
	// refFileHardLimit is the per-file token error threshold for reference files.
	refFileHardLimit = 25_000

	// refTotalSoftLimit is the aggregate token warning threshold across all reference files.
	refTotalSoftLimit = 25_000
	// refTotalHardLimit is the aggregate token error threshold across all reference files.
	refTotalHardLimit = 50_000

	// otherTotalSoftLimit is the aggregate token warning threshold for non-standard files.
	otherTotalSoftLimit = 25_000
	// otherTotalHardLimit is the aggregate token error threshold for non-standard files.
	otherTotalHardLimit = 100_000
)

var (
	encoderOnce sync.Once
	cachedEnc   tokenizer.Codec
	encoderErr  error
)

func getEncoder() (tokenizer.Codec, error) {
	encoderOnce.Do(func() {
		cachedEnc, encoderErr = tokenizer.Get(tokenizer.O200kBase)
	})
	return cachedEnc, encoderErr
}

// CheckTokens counts tokens for the SKILL.md body, reference files, asset files,
// and non-standard files. It returns validation results, standard token counts,
// and non-standard ("other") token counts.
func CheckTokens(dir, body string, opts Options) ([]types.Result, []types.TokenCount, []types.TokenCount) {
	ctx := types.ResultContext{Category: "Tokens"}
	var results []types.Result
	var counts []types.TokenCount

	enc, err := getEncoder()
	if err != nil {
		results = append(results, ctx.Errorf("failed to initialize tokenizer: %v", err))
		return results, counts, nil
	}

	// Count SKILL.md body tokens
	bodyTokens, _, _ := enc.Encode(body)
	bodyCount := len(bodyTokens)
	counts = append(counts, types.TokenCount{File: "SKILL.md body", Tokens: bodyCount})

	// Warn if body exceeds 5000 tokens
	if bodyCount > 5000 {
		results = append(results, ctx.WarnFilef("SKILL.md", "SKILL.md body is %d tokens (spec recommends < 5000)", bodyCount))
	}

	// Warn if SKILL.md exceeds 500 lines
	lineCount := strings.Count(body, "\n") + 1
	if lineCount > 500 {
		results = append(results, ctx.WarnFilef("SKILL.md", "SKILL.md body is %d lines (spec recommends < 500)", lineCount))
	}

	// Count tokens for files in references/
	refTotal := 0
	refsDir := filepath.Join(dir, "references")
	if entries, err := os.ReadDir(refsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if !entry.Type().IsRegular() {
				relPath := "references/" + entry.Name()
				results = append(results, ctx.WarnFilef(relPath, "skipping non-regular file: %s", relPath))
				continue
			}
			path := filepath.Join(refsDir, entry.Name())
			data, err := readFileWithCap(dir, path)
			if err != nil {
				relPath := "references/" + entry.Name()
				results = append(results, ctx.WarnFilef(relPath, "could not read %s: %v", relPath, err))
				continue
			}
			tokens, _, _ := enc.Encode(string(data))
			fileTokens := len(tokens)
			relPath := "references/" + entry.Name()
			counts = append(counts, types.TokenCount{
				File:   relPath,
				Tokens: fileTokens,
			})
			refTotal += fileTokens

			// Per-file limits
			if fileTokens > refFileHardLimit {
				results = append(results, ctx.ErrorFilef(relPath,
					"%s is %d tokens — this will consume 12-20%% of a typical context window "+
						"and meaningfully degrade agent performance; split into smaller focused files",
					relPath, fileTokens,
				))
			} else if fileTokens > refFileSoftLimit {
				results = append(results, ctx.WarnFilef(relPath,
					"%s is %d tokens — consider splitting into smaller focused files "+
						"so agents load only what they need",
					relPath, fileTokens,
				))
			}
		}
	}

	// When flat layouts are allowed, root-level text files are treated as
	// standard content (like references/) rather than "other" files.
	if opts.AllowFlatLayouts {
		rootCounts := countRootFiles(dir, enc)
		for _, rc := range rootCounts {
			counts = append(counts, rc)
			refTotal += rc.Tokens

			if rc.Tokens > refFileHardLimit {
				results = append(results, ctx.ErrorFilef(rc.File,
					"%s is %d tokens — this will consume 12-20%% of a typical context window "+
						"and meaningfully degrade agent performance; split into smaller focused files",
					rc.File, rc.Tokens,
				))
			} else if rc.Tokens > refFileSoftLimit {
				results = append(results, ctx.WarnFilef(rc.File,
					"%s is %d tokens — consider splitting into smaller focused files "+
						"so agents load only what they need",
					rc.File, rc.Tokens,
				))
			}
		}
	}

	// Aggregate reference limits (includes root files when flat layouts accepted)
	if refTotal > refTotalHardLimit {
		results = append(results, ctx.Errorf(
			"total reference files: %d tokens — this will consume 25-40%% of a typical "+
				"context window; reduce content or split into a skill with fewer references",
			refTotal,
		))
	} else if refTotal > refTotalSoftLimit {
		results = append(results, ctx.Warnf(
			"total reference files: %d tokens — agents may load multiple references "+
				"in one session, consider whether all this content is essential",
			refTotal,
		))
	}

	// Count tokens in non-standard files
	exclusions := newTokenPathExclusions(opts.ExcludeTokenPaths)
	otherCounts := countOtherFiles(dir, enc, opts, exclusions)

	for _, excludedPath := range exclusions.appliedPaths() {
		results = append(results, ctx.Infof("excluded from token accounting: %s", excludedPath))
	}

	// Check other-files aggregate limits
	otherTotal := 0
	for _, c := range otherCounts {
		otherTotal += c.Tokens
	}
	if otherTotal > otherTotalHardLimit {
		results = append(results, ctx.Errorf(
			"non-standard files total %d tokens — if an agent loads these, "+
				"they will consume most of the context window and severely degrade performance; "+
				"move essential content into references/ or remove unnecessary files",
			otherTotal,
		))
	} else if otherTotal > otherTotalSoftLimit {
		results = append(results, ctx.Warnf(
			"non-standard files total %d tokens — if an agent loads these, "+
				"they could consume a significant portion of the context window; "+
				"consider moving essential content into references/ or removing unnecessary files",
			otherTotal,
		))
	}

	// Count tokens in text-based asset files
	assetCounts := countAssetFiles(dir, enc)
	counts = append(counts, assetCounts...)

	return results, counts, otherCounts
}

// binaryExtensions lists file extensions that are skipped during token counting
// because they are not text-based content.
var binaryExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true,
	".ico": true, ".svg": true, ".webp": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".7z": true, ".rar": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".bin": true,
	".mp3": true, ".mp4": true, ".wav": true, ".avi": true, ".mov": true,
	".woff": true, ".woff2": true, ".ttf": true, ".eot": true, ".otf": true,
}

// standardRootFiles lists root-level files already counted in the main token
// table, so they are excluded from the "other files" count.
var standardRootFiles = map[string]bool{
	"skill.md": true,
}

// standardDirs lists directories already handled by the standard skill
// structure, so their contents are excluded from the "other files" count.
var standardDirs = map[string]bool{
	"references": true,
	"scripts":    true,
	"assets":     true,
}

// textAssetExtensions lists file extensions in assets/ that are text-based and
// likely loaded into LLM context (templates, guides, configs). These are
// included in token counting.
var textAssetExtensions = map[string]bool{
	".md":       true,
	".tex":      true,
	".py":       true,
	".yaml":     true,
	".yml":      true,
	".tsx":      true,
	".ts":       true,
	".jsx":      true,
	".sty":      true,
	".mplstyle": true,
	".ipynb":    true,
}

func countAssetFiles(dir string, enc tokenizer.Codec) []types.TokenCount {
	var counts []types.TokenCount
	assetsDir := filepath.Join(dir, "assets")

	_ = filepath.Walk(assetsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && path != assetsDir {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(info.Name()))
		if !textAssetExtensions[ext] {
			return nil
		}
		data, err := readFileWithCap(dir, path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		tokens, _, _ := enc.Encode(string(data))
		counts = append(counts, types.TokenCount{File: filepath.ToSlash(rel), Tokens: len(tokens)})
		return nil
	})

	return counts
}

type tokenPathExclusions struct {
	paths   []string
	applied map[string]bool
}

func newTokenPathExclusions(paths []string) *tokenPathExclusions {
	normalized := normalizedRelativePaths(paths)
	return &tokenPathExclusions{
		paths:   normalized,
		applied: make(map[string]bool, len(normalized)),
	}
}

func (e *tokenPathExclusions) excludes(candidate string) bool {
	candidate = filepath.ToSlash(candidate)
	for _, excludedPath := range e.paths {
		if pathInSubtree(candidate, excludedPath) {
			e.applied[excludedPath] = true
			return true
		}
	}
	return false
}

func (e *tokenPathExclusions) appliedPaths() []string {
	var paths []string
	for _, excludedPath := range e.paths {
		if e.applied[excludedPath] {
			paths = append(paths, excludedPath)
		}
	}
	return paths
}

func countOtherFiles(dir string, enc tokenizer.Codec, opts Options, exclusions *tokenPathExclusions) []types.TokenCount {
	var counts []types.TokenCount

	entries, err := os.ReadDir(dir)
	if err != nil {
		return counts
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}

		if entry.IsDir() {
			if standardDirs[strings.ToLower(name)] {
				continue
			}
			if exclusions.excludes(name) {
				continue
			}
			// Walk files in unknown directory
			counts = append(counts, countFilesInDir(dir, name, enc, exclusions)...)
		} else {
			if !entry.Type().IsRegular() {
				continue
			}
			if standardRootFiles[strings.ToLower(name)] || opts.AllowFlatLayouts {
				continue
			}
			if exclusions.excludes(name) {
				continue
			}
			if binaryExtensions[strings.ToLower(filepath.Ext(name))] {
				continue
			}
			data, err := readFileWithCap(dir, filepath.Join(dir, name))
			if err != nil {
				continue
			}
			tokens, _, _ := enc.Encode(string(data))
			counts = append(counts, types.TokenCount{File: name, Tokens: len(tokens)})
		}
	}

	return counts
}

func countFilesInDir(rootDir, dirName string, enc tokenizer.Codec, exclusions *tokenPathExclusions) []types.TokenCount {
	var counts []types.TokenCount
	fullDir := filepath.Join(rootDir, dirName)

	_ = filepath.Walk(fullDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && path != fullDir {
				return filepath.SkipDir
			}
			if path != fullDir {
				rel, relErr := filepath.Rel(rootDir, path)
				if relErr == nil && exclusions.excludes(rel) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}
		rel, relErr := filepath.Rel(rootDir, path)
		if relErr != nil {
			return nil
		}
		if exclusions.excludes(rel) {
			return nil
		}
		if binaryExtensions[strings.ToLower(filepath.Ext(info.Name()))] {
			return nil
		}
		data, err := readFileWithCap(rootDir, path)
		if err != nil {
			return nil
		}
		tokens, _, _ := enc.Encode(string(data))
		counts = append(counts, types.TokenCount{File: filepath.ToSlash(rel), Tokens: len(tokens)})
		return nil
	})

	return counts
}

// countRootFiles counts tokens in non-SKILL.md text files at the skill root.
// Used when flat layouts are allowed to treat these as standard content.
func countRootFiles(dir string, enc tokenizer.Codec) []types.TokenCount {
	var counts []types.TokenCount
	entries, err := os.ReadDir(dir)
	if err != nil {
		return counts
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		if !entry.Type().IsRegular() {
			continue
		}
		if standardRootFiles[strings.ToLower(name)] {
			continue
		}
		if binaryExtensions[strings.ToLower(filepath.Ext(name))] {
			continue
		}
		data, err := readFileWithCap(dir, filepath.Join(dir, name))
		if err != nil {
			continue
		}
		tokens, _, _ := enc.Encode(string(data))
		counts = append(counts, types.TokenCount{File: name, Tokens: len(tokens)})
	}
	return counts
}
