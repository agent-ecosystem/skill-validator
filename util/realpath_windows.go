//go:build windows

package util

import (
	"fmt"
	"os"
	"path/filepath"
)

// maxLinkHops bounds the links followed, as filepath.EvalSymlinks does, so
// a link that leads back into its own path cannot loop forever.
const maxLinkHops = 255

// evalLinks walks path from its volume, replacing each symlink or junction
// with its target as it is met and restarting from the volume, so a link
// found inside another link's target is resolved too. Once no element is a
// link, filepath.EvalSymlinks normalizes case and short names, which is all
// it has left to do. Other reparse points, such as cloud file placeholders,
// are not links and are left to it as well.
func evalLinks(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	hops := 0
	vol := len(filepath.VolumeName(path))
	for i := vol + 1; i <= len(path); i++ {
		if i < len(path) && !os.IsPathSeparator(path[i]) {
			continue
		}
		prefix := path[:i]
		fi, err := os.Lstat(prefix)
		if err != nil {
			return "", err
		}
		var target string
		switch {
		case fi.Mode()&os.ModeSymlink != 0:
			target, err = os.Readlink(prefix)
			if err != nil {
				return "", err
			}
			switch {
			case filepath.IsAbs(target):
			case len(target) > 0 && os.IsPathSeparator(target[0]):
				// Relative to the volume of the link.
				target = filepath.VolumeName(prefix) + target
			default:
				// Relative to the directory holding the link.
				target = filepath.Join(filepath.Dir(prefix), target)
			}
		case fi.Mode()&os.ModeIrregular != 0:
			// A junction's target is always absolute. Readlink fails for
			// reparse points that are not links.
			target, err = os.Readlink(prefix)
			if err != nil {
				continue
			}
		default:
			continue
		}
		hops++
		if hops > maxLinkHops {
			return "", fmt.Errorf("too many links: %s", path)
		}
		path = filepath.Join(target, path[i:])
		vol = len(filepath.VolumeName(path))
		i = vol
	}
	return filepath.EvalSymlinks(path)
}
