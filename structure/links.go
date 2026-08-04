package structure

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/agent-ecosystem/skill-validator/links"
	"github.com/agent-ecosystem/skill-validator/types"
	"github.com/agent-ecosystem/skill-validator/util"
)

// CheckInternalLinks validates relative (internal) links in the skill body.
// Broken internal links indicate a structural problem: the skill references
// files that don't exist in the package.
func CheckInternalLinks(dir, body string) []types.Result {
	ctx := types.ResultContext{Category: "Structure", File: "SKILL.md"}
	allLinks := links.ExtractLinks(body)
	if len(allLinks) == 0 {
		return nil
	}

	var results []types.Result

	for _, link := range allLinks {
		// Skip template URLs containing {placeholder} variables (RFC 6570 URI Templates)
		if strings.Contains(link, "{") {
			continue
		}
		// Skip HTTP(S) links — those are external
		if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") {
			continue
		}
		// Skip mailto and anchor links
		if strings.HasPrefix(link, "mailto:") || strings.HasPrefix(link, "#") {
			continue
		}
		// Strip fragment identifier (e.g. "guide.md#heading" → "guide.md")
		link, _, _ = strings.Cut(link, "#")
		if link == "" {
			continue
		}
		// Relative link — check file existence
		resolved := filepath.Clean(filepath.Join(dir, link))
		// Block path traversal: the resolved path must stay inside the skill directory.
		if !strings.HasPrefix(resolved, filepath.Clean(dir)+string(filepath.Separator)) {
			results = append(results, ctx.Errorf("internal link escapes skill directory: %s", link))
			continue
		}
		if _, err := os.Stat(resolved); os.IsNotExist(err) {
			results = append(results, ctx.Errorf("broken internal link: %s (file not found)", link))
			continue
		}
		// The syntactic check above cannot see symlinks: a path that looks
		// internal may still resolve outside the skill directory.
		inside, err := util.ResolvesWithin(dir, resolved)
		if err != nil || !inside {
			results = append(results, ctx.Errorf("internal link escapes skill directory: %s (resolves outside the skill package)", link))
			continue
		}
		results = append(results, ctx.Passf("internal link: %s (exists)", link))
	}

	return results
}
