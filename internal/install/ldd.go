package install

import (
	"os"
	"os/exec"
	"strings"
)

// missingLibs runs ldd on ELF binaries and returns libraries
// reported as "not found". Scripts and other files yield nothing.
func missingLibs(exe string) []string {
	f, err := os.Open(exe)
	if err != nil {
		return nil
	}
	magic := make([]byte, 4)
	_, err = f.Read(magic)
	f.Close()
	if err != nil || string(magic) != "\x7fELF" {
		return nil
	}
	out, err := exec.Command("ldd", exe).CombinedOutput()
	if err != nil && len(out) == 0 {
		return nil
	}
	var missing []string
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "not found") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			missing = append(missing, fields[0])
		}
	}
	return missing
}
