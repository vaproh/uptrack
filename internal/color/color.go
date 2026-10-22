// Package color provides minimal ANSI styling. All styling decisions
// live here; the CLI layer only picks a style per message.
package color

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

const (
	reset  = "\x1b[0m"
	bold   = "\x1b[1m"
	dim    = "\x1b[2m"
	red    = "\x1b[31m"
	green  = "\x1b[32m"
	yellow = "\x1b[33m"
	cyan   = "\x1b[36m"
)

var (
	mu      sync.Mutex
	forced  = false
	enabled = -1 // -1 means autodetect
)

// SetEnabled overrides autodetection. Pass false for --no-color.
func SetEnabled(on bool) {
	mu.Lock()
	defer mu.Unlock()
	forced = true
	if on {
		enabled = 1
	} else {
		enabled = 0
	}
}

// Enabled reports whether styling should be applied.
func Enabled() bool {
	mu.Lock()
	defer mu.Unlock()
	if enabled != -1 {
		return enabled == 1
	}
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		enabled = 0
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		enabled = 0
		return false
	}
	if fi.Mode()&os.ModeCharDevice == 0 {
		enabled = 0
		return false
	}
	if strings.ToLower(os.Getenv("TERM")) == "dumb" {
		enabled = 0
		return false
	}
	enabled = 1
	return true
}

func paint(code, s string) string {
	if !Enabled() {
		return s
	}
	return code + s + reset
}

// Bold marks section headers.
func Bold(s string) string { return paint(bold, s) }

// Dim marks labels and already-current versions.
func Dim(s string) string { return paint(dim, s) }

// Green marks available updates.
func Green(s string) string { return paint(green, s) }

// Yellow marks prereleases, unverified downloads and warnings.
func Yellow(s string) string { return paint(yellow, s) }

// Red marks failures.
func Red(s string) string { return paint(red, s) }

// Cyan marks asset paths and version values.
func Cyan(s string) string { return paint(cyan, s) }

// Sprintf formats then paints.
func Sprintf(fn func(string) string, format string, args ...any) string {
	return fn(fmt.Sprintf(format, args...))
}
