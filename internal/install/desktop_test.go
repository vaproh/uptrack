package install

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vaproh/uptrack/internal/asset"
	"github.com/vaproh/uptrack/internal/config"
	"github.com/vaproh/uptrack/internal/gh"
)

func testDataHome(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "share")
	t.Setenv("XDG_DATA_HOME", dir)
	return dir
}

func TestDebDesktop(t *testing.T) {
	data := debFixture(t)
	srv, sum := serve(t, data)
	share := testDataHome(t)
	root := t.TempDir()
	a := gh.Asset{Name: "hello_1.0_amd64.deb", Size: int64(len(data)), Digest: sum, DownloadURL: srv.URL + "/hello.deb"}
	req := Request{
		App:     config.App{Name: "hello", Repo: "o/hello", Channel: "stable"},
		Sel:     asset.Selection{Asset: a, Kind: asset.KindDeb},
		Tag:     "v1.0",
		DestDir: filepath.Join(root, "opt", "hello"),
		BinDir:  filepath.Join(root, "bin"),
	}
	res, err := Install(context.Background(), req)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if len(res.DesktopFiles) != 1 {
		t.Fatalf("want 1 desktop file, got %v", res.DesktopFiles)
	}
	raw, err := os.ReadFile(res.DesktopFiles[0])
	if err != nil {
		t.Fatal(err)
	}
	wantExec := "Exec=" + filepath.Join(req.DestDir, "usr", "bin", "hello")
	if !strings.Contains(string(raw), wantExec) {
		t.Fatalf("Exec not rewritten:\n%s", raw)
	}
	if !strings.HasPrefix(res.DesktopFiles[0], filepath.Join(share, "applications")+string(os.PathSeparator)) {
		t.Fatalf("desktop file outside data dir: %s", res.DesktopFiles[0])
	}
	if err := RemoveDesktopFiles(res.DesktopFiles); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(res.DesktopFiles[0]); !os.IsNotExist(err) {
		t.Fatal("desktop file not removed")
	}
}

const fakeAppImage = `#!/bin/sh
if [ "$1" = "--appimage-extract" ]; then
	mkdir -p squashfs-root
	printf '[Desktop Entry]\nName=tool\nExec=tool\nIcon=tool\n' > squashfs-root/tool.desktop
	printf 'icon' > squashfs-root/tool.png
	exit 0
fi
exit 1
`

func installFakeAppImage(t *testing.T, script string) (Result, string) {
	t.Helper()
	share := testDataHome(t)
	root := t.TempDir()
	tool := filepath.Join(root, "tool.AppImage")
	if err := os.WriteFile(tool, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(tool)
	if err != nil {
		t.Fatal(err)
	}
	srv, sum := serve(t, raw)
	a := gh.Asset{Name: "tool.AppImage", Size: int64(len(raw)), Digest: sum, DownloadURL: srv.URL + "/tool"}
	req := Request{
		App:     config.App{Name: "tool", Repo: "o/tool", Channel: "stable"},
		Sel:     asset.Selection{Asset: a, Kind: asset.KindAppImage},
		Tag:     "v1.0",
		DestDir: filepath.Join(root, "opt", "tool"),
		BinDir:  filepath.Join(root, "bin"),
	}
	res, err := Install(context.Background(), req)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	return res, share
}

func TestAppImageDesktop(t *testing.T) {
	res, _ := installFakeAppImage(t, fakeAppImage)
	if len(res.DesktopFiles) != 1 {
		t.Fatalf("want 1 desktop file, got %v", res.DesktopFiles)
	}
	raw, err := os.ReadFile(res.DesktopFiles[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Exec=") {
		t.Fatalf("no Exec line:\n%s", raw)
	}
}

func TestAppImageDesktopSkipped(t *testing.T) {
	res, _ := installFakeAppImage(t, "#!/bin/sh\nexit 1\n")
	if len(res.DesktopFiles) != 0 {
		t.Fatalf("want no desktop files, got %v", res.DesktopFiles)
	}
	if res.DesktopNote == "" {
		t.Fatal("want a note explaining the skip")
	}
}

func TestRewriteIcon(t *testing.T) {
	tree := t.TempDir()
	share := testDataHome(t)
	iconDir := filepath.Join(tree, "usr", "share", "pixmaps")
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(iconDir, "tool.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := rewriteDesktop("[Desktop Entry]\nName=tool\nExec=/usr/bin/tool\nIcon=tool\n", tree, "/opt/tool/tool", "tool")
	if !strings.Contains(body, "Exec=/opt/tool/tool") {
		t.Fatalf("Exec not rewritten:\n%s", body)
	}
	if !strings.Contains(body, "Icon="+filepath.Join(share, "icons", "tool-tool.png")) {
		t.Fatalf("Icon not resolved:\n%s", body)
	}
}
