package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vaproh/uptrack/internal/config"
)

func TestUninstallFlowKeepTracking(t *testing.T) {
	c := testCtx(t)
	c.assume = true
	dest := filepath.Join(c.paths.OptDir, "tool")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "tool"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(c.paths.BinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(c.paths.BinDir, "tool")
	if err := os.Symlink(filepath.Join(dest, "tool"), link); err != nil {
		t.Fatal(err)
	}
	desk := filepath.Join(t.TempDir(), "tool.desktop")
	if err := os.WriteFile(desk, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := config.App{Name: "tool", Repo: "o/tool", Channel: "stable"}
	c.cfg.Apps = []config.App{app}
	c.state.Apps["tool"] = config.AppState{
		InstalledVersion: "v1.0",
		ExeRel:           "tool",
		DesktopFiles:     []string{desk},
	}

	// Keep tracking: files and launchers go, entry and coherent state stay.
	err := uninstallFlow(c, "tool", false, false, true)
	if err != nil {
		t.Fatalf("uninstallFlow: %v", err)
	}
	if _, err := c.cfg.Find("tool"); err != nil {
		t.Fatal("tracking should be kept")
	}
	for _, p := range []string{dest, link, desk} {
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Fatalf("should be gone: %s", p)
		}
	}
	st := c.state.Apps["tool"]
	if st.InstalledVersion != "" || st.ExeRel != "" || len(st.DesktopFiles) != 0 {
		t.Fatalf("state not coherent: %+v", st)
	}
}

func TestUninstallFlowKeepFlags(t *testing.T) {
	c := testCtx(t)
	c.assume = true
	dest := filepath.Join(c.paths.OptDir, "tool")
	os.MkdirAll(dest, 0o755)
	os.WriteFile(filepath.Join(dest, "tool"), []byte("x"), 0o755)
	app := config.App{Name: "tool", Repo: "o/tool", Channel: "stable"}
	c.cfg.Apps = []config.App{app}
	c.state.Apps["tool"] = config.AppState{InstalledVersion: "v1.0", ExeRel: "tool"}

	// Keep files: entry goes, files stay.
	err := uninstallFlow(c, "tool", true, false, false)
	if err != nil {
		t.Fatalf("uninstallFlow: %v", err)
	}
	if _, err := c.cfg.Find("tool"); err == nil {
		t.Fatal("tracking should be gone")
	}
	if _, err := os.Lstat(dest); err != nil {
		t.Fatalf("files should be kept: %v", err)
	}
}
