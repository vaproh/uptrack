// Package platform detects host properties that constrain asset
// selection: OS, CPU architecture and C library.
package platform

import (
	"os"
	"runtime"
	"strings"
)

// Host describes the current machine.
type Host struct {
	OS   string // runtime.GOOS
	Arch string // normalized: amd64, arm64, 386, arm
	Libc string // glibc or musl
}

// Detect returns the current host description.
func Detect() Host {
	return Host{
		OS:   runtime.GOOS,
		Arch: NormalizeArch(runtime.GOARCH),
		Libc: DetectLibc(),
	}
}

// NormalizeArch maps runtime arch names and common asset tokens
// to a small family set. Unknown values pass through unchanged.
func NormalizeArch(a string) string {
	switch strings.ToLower(a) {
	case "amd64", "x86_64", "x86-64", "x64":
		return "amd64"
	case "arm64", "aarch64":
		return "arm64"
	case "386", "i386", "i686", "x86":
		return "386"
	case "arm", "armv6", "armv7", "armv7l", "armhf":
		return "arm"
	default:
		return strings.ToLower(a)
	}
}

// DetectLibc reports musl when the host uses musl, glibc otherwise.
// It checks for the musl dynamic loader, the Alpine release file,
// and the ldd version string, in that order.
func DetectLibc() string {
	entries, err := os.ReadDir("/lib")
	if err == nil {
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "ld-musl-") {
				return "musl"
			}
		}
	}
	raw, err := os.ReadFile("/etc/alpine-release")
	if err == nil && len(raw) > 0 {
		return "musl"
	}
	out, err := lddVersion()
	if err == nil && strings.Contains(strings.ToLower(out), "musl") {
		return "musl"
	}
	return "glibc"
}
