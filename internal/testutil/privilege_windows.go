//go:build windows

package testutil

import (
	"errors"
	"syscall"
)

func isPrivilegeError(err error) bool {
	return errors.Is(err, syscall.ERROR_PRIVILEGE_NOT_HELD)
}
