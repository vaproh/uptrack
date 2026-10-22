package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vaproh/uptrack/internal/config"
)

func TestUninstallSelfKeepsApps(t *testing.T) {
	c := testCtx(t)
	exe := filepath.Join(t.TempDir(), "uptrack")
	if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{c.paths.ConfigFile, c.paths.StateFile} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	appDest := filepath.Join(c.paths.OptDir, "tool")
	if err := os.MkdirAll(appDest, 0o755); err != nil {
		t.Fatal(err)
	}
	c.cfg.Apps = []config.App{{Name: "tool", Repo: "o/tool", Channel: "stable"}}

	removed, err := uninstallSelf(c, exe, false)
	if err != nil {
		t.Fatalf("uninstallSelf: %v", err)
	}
	if len(removed) == 0 {
		t.Fatal("want removed paths reported")
	}
	for _, p := range []string{exe, c.paths.ConfigFile, c.paths.StateFile} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Fatalf("should be gone: %s", p)
		}
	}
	if _, err := os.Lstat(appDest); err != nil {
		t.Fatalf("apps must be kept by default: %v", err)
	}
}

func TestUninstallSelfAll(t *testing.T) {
	c := testCtx(t)
	exe := filepath.Join(t.TempDir(), "uptrack")
	os.WriteFile(exe, []byte("x"), 0o755)
	for _, p := range []string{c.paths.ConfigFile, c.paths.StateFile} {
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("{}"), 0o644)
	}
	appDest := filepath.Join(c.paths.OptDir, "tool")
	os.MkdirAll(appDest, 0o755)
	os.MkdirAll(c.paths.BinDir, 0o755)
	link := filepath.Join(c.paths.BinDir, "tool")
	os.Symlink(filepath.Join(appDest, "tool"), link)
	desk := filepath.Join(t.TempDir(), "tool.desktop")
	os.WriteFile(desk, []byte("x"), 0o644)
	c.cfg.Apps = []config.App{{Name: "tool", Repo: "o/tool", Channel: "stable"}}
	c.state.Apps["tool"] = config.AppState{DesktopFiles: []string{desk}}

	_, err := uninstallSelf(c, exe, true)
	if err != nil {
		t.Fatalf("uninstallSelf: %v", err)
	}
	for _, p := range []string{exe, appDest, link, desk} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Fatalf("should be gone: %s", p)
		}
	}
}
