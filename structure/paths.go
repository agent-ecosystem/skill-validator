package structure

import (
	"fmt"
	"path"
	"strings"
)

// NormalizeRelativePaths validates paths relative to a skill root and returns
// them with slash separators. The normalized form is portable across operating
// systems and safe to use for path-boundary comparisons.
func NormalizeRelativePaths(paths []string) ([]string, error) {
	normalized := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))

	for _, raw := range paths {
		value, err := normalizeRelativePath(raw)
		if err != nil {
			return nil, err
		}
		if !seen[value] {
			normalized = append(normalized, value)
			seen[value] = true
		}
	}

	return normalized, nil
}

func normalizedRelativePaths(paths []string) []string {
	normalized := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, raw := range paths {
		value, err := normalizeRelativePath(raw)
		if err == nil && !seen[value] {
			normalized = append(normalized, value)
			seen[value] = true
		}
	}
	return normalized
}

func normalizeRelativePath(raw string) (string, error) {
	value := strings.TrimSpace(strings.ReplaceAll(raw, `\`, "/"))
	if value == "" {
		return "", fmt.Errorf("path must not be empty")
	}
	if strings.HasPrefix(value, "/") || hasWindowsVolume(value) {
		return "", fmt.Errorf("%q must be relative to the skill root", raw)
	}

	value = path.Clean(value)
	if value == "." {
		return "", fmt.Errorf("%q must identify a path below the skill root", raw)
	}
	if value == ".." || strings.HasPrefix(value, "../") {
		return "", fmt.Errorf("%q escapes the skill root", raw)
	}

	return value, nil
}

func hasWindowsVolume(value string) bool {
	return len(value) >= 2 &&
		((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) &&
		value[1] == ':'
}

func pathInSubtree(candidate, subtree string) bool {
	return candidate == subtree || strings.HasPrefix(candidate, subtree+"/")
}
