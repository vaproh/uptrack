package platform

import (
	"os/exec"
)

// lddVersion runs ldd --version; split out so tests can reason
// about DetectLibc without it. Kept tiny on purpose.
func lddVersion() (string, error) {
	out, err := exec.Command("ldd", "--version").CombinedOutput()
	if err != nil {
		// musl ldd exits nonzero and prints to stdout anyway.
		if len(out) == 0 {
			return "", err
		}
	}
	return string(out), nil
}
