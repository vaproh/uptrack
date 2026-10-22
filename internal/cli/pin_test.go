package cli

import (
	"testing"

	"github.com/vaproh/uptrack/internal/config"
	"github.com/vaproh/uptrack/internal/gh"
)

func rel(tag string) *gh.Release { return &gh.Release{TagName: tag} }

func TestSyncTag(t *testing.T) {
	cases := []struct {
		name      string
		app       config.App
		installed string
		rel       *gh.Release
		want      string
	}{
		{"missing installs latest", config.App{}, "", rel("v2.0"), "v2.0"},
		{"current skips", config.App{}, "v2.0", rel("v2.0"), ""},
		{"outdated upgrades", config.App{}, "v1.0", rel("v2.0"), "v2.0"},
		{"no release skips", config.App{}, "", nil, ""},
		{"pinned current skips", config.App{Pin: "v1.0"}, "v1.0", rel("v2.0"), ""},
		{"pinned missing installs pin", config.App{Pin: "v1.0"}, "", rel("v2.0"), "v1.0"},
		{"pinned outdated installs pin", config.App{Pin: "v1.0"}, "v0.9", rel("v2.0"), "v1.0"},
	}
	for _, c := range cases {
		if got := syncTag(c.app, c.installed, c.rel); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
		if c.app.Pin == "" {
			continue
		}
	}
}

func TestNeedsAttentionPin(t *testing.T) {
	pinned := config.App{Pin: "v1.0"}
	if needsAttention(pinned, "v1.0", rel("v2.0")) {
		t.Error("pinned current app should not need attention")
	}
	if !needsAttention(pinned, "v0.9", rel("v2.0")) {
		t.Error("pinned app off its pin should need attention")
	}
	if !needsAttention(pinned, "", rel("v2.0")) {
		t.Error("pinned missing app should need attention")
	}
	plain := config.App{}
	if !needsAttention(plain, "", rel("v2.0")) {
		t.Error("missing app should need attention")
	}
	if needsAttention(plain, "v2.0", rel("v2.0")) {
		t.Error("current app should not need attention")
	}
}
