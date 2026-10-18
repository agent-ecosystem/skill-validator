//go:build !windows

package testutil

func isPrivilegeError(error) bool { return false }
