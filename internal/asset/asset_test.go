package asset

import (
	"errors"
	"testing"

	"uptrack/internal/config"
	"uptrack/internal/gh"
	"uptrack/internal/platform"
)

var host = platform.Host{OS: "linux", Arch: "amd64", Libc: "glibc"}

// Real opencode v1.19.3 asset names.
var opencodeAssets = []gh.Asset{
	{Name: "latest-linux-arm64.yml"},
	{Name: "latest-linux.yml"},
	{Name: "latest-mac.yml"},
	{Name: "latest.json"},
	{Name: "latest.yml"},
	{Name: "opencode-darwin-arm64.zip"},
	{Name: "opencode-darwin-x64-baseline.zip"},
	{Name: "opencode-darwin-x64.zip"},
	{Name: "opencode-desktop-linux-aarch64.rpm"},
	{Name: "opencode-desktop-linux-amd64.deb"},
	{Name: "opencode-desktop-linux-arm64.AppImage"},
	{Name: "opencode-desktop-linux-arm64.deb"},
	{Name: "opencode-desktop-linux-x86_64.AppImage"},
	{Name: "opencode-desktop-linux-x86_64.rpm"},
	{Name: "opencode-linux-arm64-musl.tar.gz"},
	{Name: "opencode-linux-arm64.tar.gz"},
	{Name: "opencode-linux-x64-baseline-musl.tar.gz"},
	{Name: "opencode-linux-x64-baseline.tar.gz"},
	{Name: "opencode-linux-x64-musl.tar.gz"},
	{Name: "opencode-linux-x64.tar.gz"},
	{Name: "opencode-windows-arm64.zip"},
	{Name: "opencode-windows-x64-baseline.zip"},
	{Name: "opencode-windows-x64.zip"},
}

func TestOpencodePick(t *testing.T) {
	sel, log, err := Select(opencodeAssets, nil, host)
	if err != nil {
		t.Fatalf("select: %v\nlog: %+v", err, log)
	}
	if sel.Asset.Name != "opencode-linux-x64.tar.gz" {
		t.Fatalf("picked %q, want opencode-linux-x64.tar.gz", sel.Asset.Name)
	}
	if sel.Kind != KindArchive || sel.Compression != "gz" {
		t.Fatalf("kind/comp: %v %v", sel.Kind, sel.Compression)
	}
}

func TestRuleExclude(t *testing.T) {
	assets := []gh.Asset{
		{Name: "tool-linux-x64.tar.gz"},
		{Name: "tool-linux-x86_64.tar.gz"},
	}
	// Both name the same system the same way: ambiguous.
	_, _, err := Select(assets, nil, host)
	var amb *AmbiguousError
	if !errors.As(err, &amb) {
		t.Fatalf("expected ambiguity, got %v", err)
	}
	if len(amb.Candidates) != 2 {
		t.Fatalf("want 2 candidates, got %d", len(amb.Candidates))
	}
	rule := &config.AssetRule{Match: "tool-linux-x64.tar.gz"}
	sel, _, err := Select(assets, rule, host)
	if err != nil {
		t.Fatalf("select with rule: %v", err)
	}
	if sel.Asset.Name != "tool-linux-x64.tar.gz" {
		t.Fatalf("picked %q", sel.Asset.Name)
	}
}

func TestFormatPreference(t *testing.T) {
	// Same logical build as raw binary and tarball: binary wins.
	assets := []gh.Asset{
		{Name: "tool-linux-amd64.tar.gz"},
		{Name: "tool-linux-amd64"},
	}
	sel, _, err := Select(assets, nil, host)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if sel.Kind != KindBinary {
		t.Fatalf("want binary, got %v (%s)", sel.Kind, sel.Asset.Name)
	}
}

func TestRejects(t *testing.T) {
	assets := []gh.Asset{
		{Name: "tool-darwin-arm64.zip"},
		{Name: "tool-windows-x64.zip"},
		{Name: "tool-linux-arm64.tar.gz"},
		{Name: "tool-linux-x64-musl.tar.gz"},
		{Name: "tool-1.0.yml"},
		{Name: "tool-1.0.dmg"},
	}
	_, log, err := Select(assets, nil, host)
	if err == nil {
		t.Fatal("expected no-match error")
	}
	if len(log) != len(assets) {
		t.Fatalf("log should cover every asset: %+v", log)
	}
}

func TestGuiDemoted(t *testing.T) {
	assets := []gh.Asset{
		{Name: "tool-linux-x64.AppImage"},
		{Name: "tool-desktop-linux-x64.AppImage"},
	}
	sel, _, err := Select(assets, nil, host)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if sel.Asset.Name != "tool-linux-x64.AppImage" {
		t.Fatalf("picked %q", sel.Asset.Name)
	}
}

func TestMuslHost(t *testing.T) {
	musl := platform.Host{OS: "linux", Arch: "amd64", Libc: "musl"}
	assets := []gh.Asset{
		{Name: "tool-linux-x64.tar.gz"},
		{Name: "tool-linux-x64-musl.tar.gz"},
	}
	// Both match; glibc-tagged? plain has no libc token so it passes too.
	// The musl one should survive and the plain one should survive: tie.
	// Give the musl asset a distinguishing edge via explicit rule instead.
	rule := &config.AssetRule{Match: "*musl*"}
	sel, _, err := Select(assets, rule, musl)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if sel.Asset.Name != "tool-linux-x64-musl.tar.gz" {
		t.Fatalf("picked %q", sel.Asset.Name)
	}
	// A gnu-tagged asset must be rejected on musl.
	gnu := []gh.Asset{{Name: "tool-linux-x64-gnu.tar.gz"}}
	_, _, err = Select(gnu, nil, musl)
	if err == nil {
		t.Fatal("expected gnu build to be rejected on musl")
	}
}
