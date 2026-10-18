//go:build windows

package testutil

import (
	"os/exec"
	"testing"
)

// Junction creates a directory junction at link leading to target. Unlike a
// symlink, a junction needs no privilege, which is why it is the common way
// to alias a directory on Windows and why Windows CI can always exercise it.
func Junction(t testing.TB, target, link string) {
	t.Helper()
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Fatalf("mklink /J %s %s: %v\n%s", link, target, err, out)
	}
}
