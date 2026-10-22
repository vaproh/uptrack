package config

import (
	"os"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := dir + "/config.yaml"
	statePath := dir + "/state.json"

	c, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("load missing: %v", err)
	}
	if len(c.Apps) != 0 {
		t.Fatalf("expected empty config, got %d apps", len(c.Apps))
	}

	c.Apps = []App{{
		Name:    "opencode",
		Repo:    "anomalyco/opencode",
		Channel: "stable",
		Asset:   &AssetRule{Match: "opencode-linux-x64*.tar.gz", Exclude: []string{"*musl*"}},
	}}
	err = SaveConfig(cfgPath, c)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	c2, err := LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	got, err := c2.Find("opencode")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Asset.Match != "opencode-linux-x64*.tar.gz" {
		t.Fatalf("match not preserved: %q", got.Asset.Match)
	}

	s, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("load missing state: %v", err)
	}
	s.Apps["opencode"] = AppState{InstalledVersion: "v1.19.2", Verified: true}
	err = SaveState(statePath, s)
	if err != nil {
		t.Fatalf("save state: %v", err)
	}
	s2, err := LoadState(statePath)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if s2.Apps["opencode"].InstalledVersion != "v1.19.2" {
		t.Fatalf("state not preserved: %+v", s2.Apps["opencode"])
	}
}

func TestMalformed(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/config.yaml"
	err := os.WriteFile(p, []byte("apps: [unclosed"), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, err = LoadConfig(p)
	if err == nil {
		t.Fatal("expected parse error")
	}
	_, err = LoadState(p)
	if err == nil {
		t.Fatal("expected state parse error")
	}
}

func TestFindMissing(t *testing.T) {
	c := &Config{}
	_, err := c.Find("nope")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}
