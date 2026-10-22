package install

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/vaproh/uptrack/internal/asset"
)

// dataDir honors XDG_DATA_HOME, defaulting to ~/.local/share.
func dataDir() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share")
}

// installDesktopFiles copies launcher files for GUI installs into the
// data dir and rewrites Exec to the installed binary. Only deb and
// AppImage installs are handled; anything else returns nothing.
// AppImage extraction needs FUSE, so a failure there is a note, never
// an error: icons must not break an install.
func installDesktopFiles(staged, exeAbs string, kind asset.Kind, appName string) (files []string, note string, err error) {
	if dataDir() == "" {
		return nil, "", fmt.Errorf("cannot locate data dir")
	}
	switch kind {
	case asset.KindDeb:
		files, err = installStagedDesktop(staged, exeAbs, appName)
		if err != nil {
			return nil, "", err
		}
		return files, "", nil
	case asset.KindAppImage:
		appFile := filepath.Join(staged, appName)
		files, note, err := extractAppImageDesktop(appFile, exeAbs, appName)
		if err != nil {
			return nil, "", err
		}
		return files, note, nil
	default:
		return nil, "", nil
	}
}

// installStagedDesktop copies .desktop files from tree into the data
// dir, rewrites Exec, resolves icons, and returns installed paths.
func installStagedDesktop(tree, exeAbs, appName string) ([]string, error) {
	found, err := desktops(tree)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, nil
	}
	appsDir := filepath.Join(dataDir(), "applications")
	err = os.MkdirAll(appsDir, 0o755)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, d := range found {
		raw, err := os.ReadFile(d)
		if err != nil {
			return nil, err
		}
		body := rewriteDesktop(string(raw), tree, exeAbs, appName)
		name := appName + "-" + filepath.Base(d)
		target := filepath.Join(appsDir, name)
		err = os.WriteFile(target, []byte(body), 0o644)
		if err != nil {
			return nil, err
		}
		out = append(out, target)
	}
	updateDesktopDB(appsDir)
	return out, nil
}

// rewriteDesktop points Exec at the installed binary and resolves the
// Icon to a copied file when the package ships one.
func rewriteDesktop(body, tree, exeAbs, appName string) string {
	var out []string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "Exec="):
			exec := strings.TrimSpace(strings.TrimPrefix(trimmed, "Exec="))
			rest := ""
			if fields := strings.Fields(exec); len(fields) > 1 {
				rest = " " + strings.Join(fields[1:], " ")
			}
			out = append(out, "Exec="+exeAbs+rest)
		case strings.HasPrefix(trimmed, "Icon="):
			icon := strings.TrimSpace(strings.TrimPrefix(trimmed, "Icon="))
			if resolved := installIcon(tree, icon, appName); resolved != "" {
				out = append(out, "Icon="+resolved)
			} else {
				out = append(out, line)
			}
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// installIcon copies a packaged icon into the data dir and returns its
// new absolute path, or "" when the package ships no matching icon.
func installIcon(tree, icon, appName string) string {
	var src string
	if strings.ContainsRune(icon, '/') {
		candidate := icon
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(tree, candidate)
		}
		if fi, err := os.Stat(candidate); err == nil && fi.Mode().IsRegular() {
			src = candidate
		}
	} else {
		for _, dir := range []string{"usr/share/icons", "usr/share/pixmaps", "opt"} {
			root := filepath.Join(tree, dir)
			found := findIcon(root, icon)
			if found != "" {
				src = found
				break
			}
		}
	}
	if src == "" {
		return ""
	}
	iconsDir := filepath.Join(dataDir(), "icons")
	os.MkdirAll(iconsDir, 0o755)
	target := filepath.Join(iconsDir, appName+"-"+filepath.Base(src))
	data, err := os.ReadFile(src)
	if err != nil {
		return ""
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return ""
	}
	return target
}

func findIcon(root, name string) string {
	var found string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || found != "" || d.IsDir() {
			return nil
		}
		base := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		if base == name {
			found = p
		}
		return nil
	})
	return found
}

// extractAppImageDesktop runs the AppImage's own extraction for
// launcher files. Anything going wrong yields a note, not an error.
func extractAppImageDesktop(appFile, exeAbs, appName string) ([]string, string, error) {
	tmp, err := os.MkdirTemp("", "uptrack-appimg-*")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(tmp)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, appFile, "--appimage-extract", "*.desktop", "*.png", "*.svg", "*.xpm")
	cmd.Dir = tmp
	if _, err := cmd.CombinedOutput(); err != nil {
		return nil, "desktop integration skipped: AppImage extraction failed (needs FUSE)", nil
	}
	root := filepath.Join(tmp, "squashfs-root")
	if _, err := os.Stat(root); err != nil {
		return nil, "desktop integration skipped: no files extracted", nil
	}
	files, err := installStagedDesktop(root, exeAbs, appName)
	if err != nil {
		return nil, "", err
	}
	return files, "", nil
}

// updateDesktopDB refreshes the launcher database when available.
func updateDesktopDB(appsDir string) {
	if bin, err := exec.LookPath("update-desktop-database"); err == nil {
		cmd := exec.Command(bin, appsDir)
		_ = cmd.Run()
	}
}

// RemoveDesktopFiles deletes previously installed launcher files.
// Missing files are not an error.
func RemoveDesktopFiles(files []string) error {
	for _, f := range files {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", f, err)
		}
	}
	return nil
}
