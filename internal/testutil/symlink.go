// Package testutil holds helpers shared by the test suites.
package testutil

import (
	"os"
	"testing"
)

// Symlink creates link pointing at target, or skips the test when the host
// refuses. Windows grants symlink creation only to elevated processes and to
// accounts with Developer Mode on. The suites used to skip every symlink
// test on Windows outright, which kept the platform's link handling out of
// CI even though the hosted Windows runners are elevated (#101).
func Symlink(t testing.TB, target, link string) {
	t.Helper()
	err := os.Symlink(target, link)
	if err == nil {
		return
	}
	if isPrivilegeError(err) {
		t.Skipf("cannot create symlinks on this host: %v", err)
	}
	t.Fatal(err)
}
