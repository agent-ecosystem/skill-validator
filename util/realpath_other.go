//go:build !windows

package util

import "path/filepath"

func evalLinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
