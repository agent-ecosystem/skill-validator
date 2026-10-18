//go:build !windows

package util

// expandJunctions is the identity where directory junctions do not exist.
func expandJunctions(path string) (string, error) { return path, nil }
