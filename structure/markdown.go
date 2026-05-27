package structure

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/agent-ecosystem/skill-validator/types"
	"github.com/agent-ecosystem/skill-validator/util"
)

// CheckMarkdown validates markdown structure in the skill.
func CheckMarkdown(dir, body string) []types.Result {
	ctx := types.ResultContext{Category: "Markdown"}
	var results []types.Result

	// Check SKILL.md body
	if line, ok := FindUnclosedFence(body); ok {
		results = append(results, ctx.ErrorAtLinef("SKILL.md", line,
			"SKILL.md has an unclosed code fence starting at line %d — this may cause agents to misinterpret everything after it as code", line))
	}

	// Check .md files in references/
	refsDir := filepath.Join(dir, "references")
	entries, err := os.ReadDir(refsDir)
	if err != nil {
		if len(results) == 0 {
			results = append(results, ctx.Pass("no unclosed code fences found"))
		}
		return results
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if !entry.Type().IsRegular() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		data, err := util.SafeReadFile(filepath.Join(refsDir, entry.Name()))
		if err != nil {
			continue
		}
		relPath := "references/" + entry.Name()
		if line, ok := FindUnclosedFence(string(data)); ok {
			results = append(results, ctx.ErrorAtLinef(relPath, line,
				"%s has an unclosed code fence starting at line %d — this may cause agents to misinterpret everything after it as code", relPath, line))
		}
	}

	if len(results) == 0 {
		results = append(results, ctx.Pass("no unclosed code fences found"))
	}

	return results
}

// FindUnclosedFence checks for unclosed code fences (``` or ~~~).
// Returns the line number of the unclosed opening fence and true, or 0 and false.
func FindUnclosedFence(content string) (int, bool) {
	lines := strings.Split(content, "\n")
	inFence := false
	fenceChar := byte(0)
	fenceLen := 0
	fenceLine := 0

	for i, line := range lines {
		// Strip up to 3 leading spaces
		stripped := line
		for range 3 {
			if len(stripped) > 0 && stripped[0] == ' ' {
				stripped = stripped[1:]
			} else {
				break
			}
		}

		if !inFence {
			if char, n := fencePrefix(stripped); n >= 3 {
				inFence = true
				fenceChar = char
				fenceLen = n
				fenceLine = i + 1
			}
		} else {
			if char, n := fencePrefix(stripped); n >= fenceLen && char == fenceChar {
				// Closing fence: rest must be only whitespace
				rest := stripped[n:]
				if strings.TrimSpace(rest) == "" {
					inFence = false
				}
			}
		}
	}

	if inFence {
		return fenceLine, true
	}
	return 0, false
}

// fencePrefix returns the fence character and its count if the line starts
// with 3+ backticks or 3+ tildes. Returns (0, 0) otherwise.
func fencePrefix(line string) (byte, int) {
	if len(line) == 0 {
		return 0, 0
	}
	ch := line[0]
	if ch != '`' && ch != '~' {
		return 0, 0
	}
	n := 0
	for n < len(line) && line[n] == ch {
		n++
	}
	if n < 3 {
		return 0, 0
	}
	return ch, n
}
