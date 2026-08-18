//go:build !windows

package core

import "os/exec"

// launchInstaller starts the downloaded setup installer as a detached child
// process. Non-Windows platforms have no elevation requirement.
func launchInstaller(path string) error {
	return exec.Command(path).Start()
}
