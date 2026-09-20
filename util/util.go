// Package util provides shared utility functions used across the
// skill-validator codebase: number formatting, pluralization, rounding,
// sorted-key extraction, and ANSI color helpers.
package util

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ErrUnsafeFile = errors.New("refusing to read unsafe file")

// ErrFileTooLarge reports a file in a skill package larger than the read
// limit. Callers that need the whole file (link scanning, fence matching)
// skip the file and say so; the token counter reads a bounded prefix
// through SafeReadFileN instead.
var ErrFileTooLarge = errors.New("file exceeds the read limit")

// MaxSkillFileBytes is the most the validator reads from any single file in
// a skill package (issue #87). A pathological multi-gigabyte file is never
// loaded whole: SafeReadFile refuses it, and SafeReadFileN returns only
// this many bytes.
const MaxSkillFileBytes int64 = 8 << 20

// FormatByteSize renders a byte count for messages: whole MiB or KiB where
// exact ("8 MiB"), plain bytes otherwise ("100 bytes").
func FormatByteSize(n int64) string {
	switch {
	case n >= 1<<20 && n%(1<<20) == 0:
		return fmt.Sprintf("%d MiB", n>>20)
	case n >= 1<<10 && n%(1<<10) == 0:
		return fmt.Sprintf("%d KiB", n>>10)
	default:
		return fmt.Sprintf("%d bytes", n)
	}
}

// SafeReadFile reads a whole file from an untrusted skill package rooted at
// root. It refuses non-regular files (symlinks, devices, pipes) and paths
// that resolve outside root after following symlinks in any parent
// directory, so a symlinked file or a symlinked directory inside the
// package cannot leak content from elsewhere on the machine. It also
// refuses, with ErrFileTooLarge, any file larger than MaxSkillFileBytes,
// without reading past that limit.
func SafeReadFile(root, path string) ([]byte, error) {
	data, truncated, err := SafeReadFileN(root, path, MaxSkillFileBytes)
	if err != nil {
		return nil, err
	}
	if truncated {
		return nil, fmt.Errorf("%w: %s is larger than %s", ErrFileTooLarge, path, FormatByteSize(MaxSkillFileBytes))
	}
	return data, nil
}

// SafeReadFileN is SafeReadFile with an explicit byte limit. It reads at
// most limit bytes and reports whether the file had more; the file is never
// read past the limit, so memory is bounded regardless of file size.
func SafeReadFileN(root, path string, limit int64) (data []byte, truncated bool, err error) {
	if err := checkSafePath(root, path); err != nil {
		return nil, false, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = f.Close() }()
	// Read one byte past the limit so truncation is detectable without a
	// second stat (the file can change between Lstat and Open).
	data, err = io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > limit {
		return data[:limit], true, nil
	}
	return data, false, nil
}

// checkSafePath applies SafeReadFile's regular-file and containment checks
// without reading anything.
func checkSafePath(root, path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: %s is not a regular file", ErrUnsafeFile, path)
	}
	inside, err := ResolvesWithin(root, path)
	if err != nil {
		return err
	}
	if !inside {
		return fmt.Errorf("%w: %s resolves outside the skill directory", ErrUnsafeFile, path)
	}
	return nil
}

// ResolvesWithin reports whether path, after resolving all symlinks in both
// arguments, remains inside root. The path must exist.
func ResolvesWithin(root, path string) (bool, error) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false, err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil {
		return false, err
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)), nil
}

func IsRegularFile(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

// --- Color constants for terminal output ---

const (
	// ColorReset disables all ANSI text attributes.
	ColorReset = "\033[0m"
	// ColorBold enables bold text.
	ColorBold = "\033[1m"
	// ColorRed sets the text color to red.
	ColorRed = "\033[31m"
	// ColorGreen sets the text color to green.
	ColorGreen = "\033[32m"
	// ColorYellow sets the text color to yellow.
	ColorYellow = "\033[33m"
	// ColorCyan sets the text color to cyan.
	ColorCyan = "\033[36m"
)

// --- Number formatting ---

// FormatNumber formats an integer with thousand-separator commas.
func FormatNumber(n int) string {
	s := fmt.Sprintf("%d", n)
	if n < 1000 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// RoundTo rounds val to the given number of decimal places.
func RoundTo(val float64, places int) float64 {
	pow := math.Pow(10, float64(places))
	return math.Round(val*pow) / pow
}

// --- Pluralization ---

// PluralS returns "s" when n != 1, empty string otherwise.
func PluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// YSuffix returns "y" when n == 1, "ies" otherwise.
func YSuffix(n int) string {
	if n == 1 {
		return "y"
	}
	return "ies"
}

// --- Path helpers ---

// SkillNameFromDir derives a skill name from a directory path.
func SkillNameFromDir(dir string) string {
	return filepath.Base(dir)
}

// --- Map helpers ---

// SortedKeys returns the keys of any map[string]V sorted alphabetically.
func SortedKeys[V any](m map[string]V) []string {
	if len(m) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
