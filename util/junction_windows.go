//go:build windows

package util

import (
	"fmt"
	"os"
	"path/filepath"
)

// maxJunctionHops bounds junction expansion so a junction that leads back
// into its own path cannot loop forever.
const maxJunctionHops = 64

// expandJunctions replaces each directory junction on path with its target,
// outermost first, until no element of the path is a junction. Symlinks are
// left for filepath.EvalSymlinks, which handles them and their relative
// targets. The result is absolute.
func expandJunctions(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	for range maxJunctionHops {
		prefix, target, err := firstJunction(path)
		if err != nil {
			return "", err
		}
		if prefix == "" {
			return path, nil
		}
		path = filepath.Join(target, path[len(prefix):])
	}
	return "", fmt.Errorf("too many levels of junctions: %s", path)
}

// firstJunction returns the shortest prefix of path that is a junction,
// with the junction's target. prefix is empty when the path holds none. A
// junction's target is always absolute. Other reparse points, such as cloud
// file placeholders, are not links and are left alone.
func firstJunction(path string) (prefix, target string, err error) {
	start := len(filepath.VolumeName(path))
	for i := start + 1; i <= len(path); i++ {
		if i < len(path) && !os.IsPathSeparator(path[i]) {
			continue
		}
		candidate := path[:i]
		fi, err := os.Lstat(candidate)
		if err != nil {
			return "", "", err
		}
		if fi.Mode()&os.ModeIrregular == 0 {
			continue
		}
		target, err := os.Readlink(candidate)
		if err != nil {
			continue
		}
		return candidate, target, nil
	}
	return "", "", nil
}
