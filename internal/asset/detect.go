package asset

import (
	"path"
	"strings"
)

// tokens splits a filename into lowercase alphanumeric tokens.
func tokens(name string) []string {
	lower := strings.ToLower(name)
	return strings.FieldsFunc(lower, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
}

func hasToken(toks []string, want string) bool {
	for _, t := range toks {
		if t == want {
			return true
		}
	}
	return false
}

// detectOS returns a GOOS name when the tokens name an OS.
func detectOS(toks []string) string {
	for _, t := range toks {
		switch t {
		case "linux":
			return "linux"
		case "darwin", "macos", "osx", "apple", "mac":
			// shortcut: token "app" belongs to AppImage, not mac apps.
			return "darwin"
		case "windows", "win", "win32", "win64", "mingw", "msvc":
			return "windows"
		case "freebsd", "openbsd", "netbsd":
			return t
		}
	}
	return ""
}

// detectArch returns a normalized arch family when tokens name one.
func detectArch(toks []string) string {
	for i, t := range toks {
		// Joined forms split apart: "x86-64" becomes "x86"+"64".
		if t == "x86" && i+1 < len(toks) && toks[i+1] == "64" {
			return "amd64"
		}
		if t == "64" && i > 0 && toks[i-1] == "x86" {
			return "amd64"
		}
		switch t {
		case "amd64", "x86_64", "x64":
			return "amd64"
		case "arm64", "aarch64":
			return "arm64"
		case "386", "i386", "i686", "x86":
			return "386"
		case "arm", "armv6", "armv7", "armv7l", "armhf":
			return "arm"
		}
	}
	return ""
}

// detectLibc returns musl, gnu or "".
func detectLibc(toks []string) string {
	for _, t := range toks {
		switch t {
		case "musl":
			return "musl"
		case "gnu", "glibc":
			return "gnu"
		}
	}
	return ""
}

// isGUI reports GUI-looking builds, which lose to plain CLI artifacts.
func isGUI(toks []string) bool {
	return hasToken(toks, "desktop") || hasToken(toks, "gui") || hasToken(toks, "app")
}

// detectFormat maps a filename to an install kind and, for archives,
// the compression. Unknown or non-installable names return false.
func detectFormat(name string) (Kind, string, bool) {
	lower := strings.ToLower(name)
	for _, suf := range []struct {
		s    string
		comp string
	}{
		{".tar.gz", "gz"}, {".tgz", "gz"},
		{".tar.bz2", "bz2"}, {".tbz2", "bz2"}, {".tbz", "bz2"},
		{".tar.xz", "xz"}, {".txz", "xz"},
		{".tar.zst", "zst"}, {".tzst", "zst"},
		{".tar", "none"},
	} {
		if strings.HasSuffix(lower, suf.s) {
			return KindArchive, suf.comp, true
		}
	}
	if strings.HasSuffix(lower, ".zip") {
		return KindArchive, "zip", true
	}
	if strings.HasSuffix(lower, ".appimage") {
		return KindAppImage, "", true
	}
	if strings.HasSuffix(lower, ".deb") {
		return KindDeb, "", true
	}
	if strings.Contains(lower, ".") {
		base := lower[strings.LastIndex(lower, ".")+1:]
		switch base {
		case "exe", "msi", "dmg", "pkg", "apk", "rpm", "snap", "run",
			"yml", "yaml", "json", "txt", "md", "sig", "asc", "pem",
			"sha256", "sha512", "blockmap", "sbom", "zsync", "AppImage":
			return "", "", false
		}
		// An unfamiliar extension is not installable; refusing beats
		// downloading something we cannot place.
		return "", "", false
	}
	return KindBinary, "", true
}

// matchGlob matches a user pattern case-insensitively.
func matchGlob(pattern, name string) (bool, error) {
	return path.Match(strings.ToLower(pattern), strings.ToLower(name))
}

func excluded(patterns []string, name string) bool {
	for _, p := range patterns {
		ok, err := matchGlob(p, name)
		if err == nil && ok {
			return true
		}
	}
	return false
}
