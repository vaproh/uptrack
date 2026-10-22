package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vaproh/uptrack/internal/config"
)

func testCtx(t *testing.T) *ctx {
	t.Helper()
	home := t.TempDir()
	paths := config.Paths{
		ConfigFile: filepath.Join(home, ".config", "uptrack", "config.yaml"),
		StateFile:  filepath.Join(home, ".local", "state", "uptrack", "state.json"),
		BinDir:     filepath.Join(home, ".local", "bin"),
		OptDir:     filepath.Join(home, ".local", "opt"),
	}
	return &ctx{
		paths: paths,
		cfg:   &config.Config{},
		state: &config.State{Apps: map[string]config.AppState{}},
	}
}

func TestDoctorHealthy(t *testing.T) {
	c := testCtx(t)
	dest := filepath.Join(c.paths.OptDir, "tool")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dest, "tool")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(c.paths.BinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(exe, filepath.Join(c.paths.BinDir, "tool")); err != nil {
		t.Fatal(err)
	}
	app := config.App{Name: "tool", Repo: "o/tool", Channel: "stable"}
	c.cfg.Apps = []config.App{app}
	c.state.Apps["tool"] = config.AppState{InstalledVersion: "v1.0", ExeRel: "tool"}
	if issues := checkApp(c, &app); len(issues) != 0 {
		t.Fatalf("want healthy, got %v", issues)
	}
}

func TestDoctorFindings(t *testing.T) {
	c := testCtx(t)
	app := config.App{Name: "ghost", Repo: "o/ghost", Channel: "stable"}
	c.cfg.Apps = []config.App{app}
	c.state.Apps["ghost"] = config.AppState{InstalledVersion: "v1.0", ExeRel: "ghost"}
	issues := checkApp(c, &app)
	if len(issues) == 0 {
		t.Fatal("want issues for missing install dir")
	}

	// Installed but untracked exe and broken symlink.
	c2 := testCtx(t)
	dest := filepath.Join(c2.paths.OptDir, "tool")
	os.MkdirAll(dest, 0o755)
	os.WriteFile(filepath.Join(dest, "tool"), []byte("x"), 0o644)
	os.MkdirAll(c2.paths.BinDir, 0o755)
	os.Symlink(filepath.Join(dest, "other"), filepath.Join(c2.paths.BinDir, "tool"))
	app2 := config.App{Name: "tool", Repo: "o/tool", Channel: "stable"}
	c2.state.Apps["tool"] = config.AppState{InstalledVersion: "v1.0", ExeRel: "tool"}
	issues = checkApp(c2, &app2)
	if len(issues) < 2 {
		t.Fatalf("want non-executable + wrong symlink issues, got %v", issues)
	}
}
