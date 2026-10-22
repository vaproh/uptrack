package install

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// findExe locates the executable inside an extracted archive.
// An explicit hint wins. Otherwise a single file at the root wins,
// then a single top-level directory is descended into, then the
// search looks for exactly one executable file. Anything else is an
// error naming install.exe as the fix.
func findExe(dir, hint string) (string, error) {
	if hint != "" {
		clean := filepath.Clean(hint)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
			return "", fmt.Errorf("exe hint %q must be relative to the install dir", hint)
		}
		full := filepath.Join(dir, clean)
		fi, err := os.Stat(full)
		if err != nil || fi.IsDir() {
			return "", fmt.Errorf("exe hint %q not found in archive", hint)
		}
		return clean, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	if len(entries) == 1 {
		if !entries[0].IsDir() {
			return entries[0].Name(), nil
		}
		sub, err := findExe(filepath.Join(dir, entries[0].Name()), "")
		if err != nil {
			return "", err
		}
		return filepath.Join(entries[0].Name(), sub), nil
	}
	exes, err := executables(dir)
	if err != nil {
		return "", err
	}
	if len(exes) == 1 {
		rel, err := filepath.Rel(dir, exes[0])
		if err != nil {
			return "", err
		}
		return rel, nil
	}
	return "", fmt.Errorf("cannot tell which file is the executable; set install.exe in the config")
}

// executables returns regular files with any exec bit set, skipping
// shared libraries and files under share/, which are data, not
// programs. shortcut: this heuristic covers release archives; it is
// not a substitute for install.exe when it fails.
func executables(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		base := filepath.Base(rel)
		if strings.Contains(base, ".so") {
			return nil
		}
		first, _, _ := strings.Cut(rel, string(os.PathSeparator))
		if first == "share" {
			return nil
		}
		out = append(out, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// findDebExe locates the program inside an extracted .deb: a .desktop
// file's Exec line first, then a lone binary in usr/bin or bin.
func findDebExe(dir, hint string) (string, error) {
	if hint != "" {
		return findExe(dir, hint)
	}
	desktops, err := desktops(dir)
	if err != nil {
		return "", err
	}
	for _, d := range desktops {
		exe, ok := desktopExec(dir, d)
		if ok {
			return exe, nil
		}
	}
	for _, bindir := range []string{"usr/bin", "bin", "usr/local/bin"} {
		full := filepath.Join(dir, bindir)
		entries, err := os.ReadDir(full)
		if err != nil {
			continue
		}
		var files []string
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			fi, err := e.Info()
			if err != nil || !fi.Mode().IsRegular() {
				continue
			}
			files = append(files, filepath.Join(bindir, e.Name()))
		}
		if len(files) == 1 {
			return files[0], nil
		}
	}
	return "", fmt.Errorf("cannot tell which file is the executable; set install.exe in the config")
}

func desktops(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".desktop") {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// desktopExec reads the Exec line and resolves it inside dir.
func desktopExec(dir, desktop string) (string, bool) {
	raw, err := os.ReadFile(desktop)
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Exec=") {
			continue
		}
		exec := strings.TrimSpace(strings.TrimPrefix(line, "Exec="))
		// Strip field codes (%f, %u...) and quoting.
		fields := strings.Fields(exec)
		if len(fields) == 0 {
			return "", false
		}
		bin := strings.Trim(fields[0], `"'`)
		for _, b := range fields[1:] {
			if !strings.HasPrefix(b, "%") && !strings.HasPrefix(b, "-") {
				bin += " " + strings.Trim(b, `"'"`)
				break
			}
		}
		if filepath.IsAbs(bin) {
			rel, err := filepath.Rel("/", bin)
			if err != nil {
				return "", false
			}
			bin = rel
		}
		if _, err := os.Stat(filepath.Join(dir, bin)); err == nil {
			return bin, true
		}
		return "", false
	}
	return "", false
}

// warnMissingLibs prints a warning when an ELF binary needs shared
// libraries the loader cannot find. Non-ELF files are ignored.
func warnMissingLibs(exe string) {
	libs := MissingLibs(exe)
	if len(libs) > 0 {
		fmt.Printf("warning: missing shared libraries: %s\n", strings.Join(libs, ", "))
		fmt.Println("install them with your system package manager")
	}
}
