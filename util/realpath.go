package util

import "path/filepath"

// RealPath returns path with every symlink and, on Windows, every directory
// junction resolved. It is filepath.EvalSymlinks extended to junctions:
// since Go 1.23 the standard library reports a junction as an irregular
// file rather than a symlink, leaves it unresolved, and fails with "not a
// directory" when one sits above the last element of a path. Without this,
// a skill reached through a junction, the usual way to alias a directory on
// Windows (#101), is found by discovery and then cannot be read. The path
// must exist.
func RealPath(path string) (string, error) {
	expanded, err := expandJunctions(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(expanded)
}
