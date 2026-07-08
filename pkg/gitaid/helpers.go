// SPDX-FileCopyrightText: (c) 2026 Rafal Zajac
// SPDX-License-Identifier: MIT

package gitaid

import (
	"errors"
	"os/exec"
)

// exitStatus returns the exit status of the error if it's an instance of
// *exec.ExitError or it has method: ExitStatus() int. It returns 0 if err is
// nil and 1 if err does not match above criteria.
func exitStatus(err error) int {
	if err == nil {
		return 0
	}

	type status interface{ ExitStatus() int }

	var es status
	if errors.As(err, &es) {
		return es.ExitStatus()
	}

	if ee, ok := errors.AsType[*exec.ExitError](err); ok {
		if ex, ok := ee.Sys().(status); ok {
			return ex.ExitStatus()
		}
	}
	return 1
}
