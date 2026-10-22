package install

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vaproh/uptrack/internal/asset"
	"github.com/vaproh/uptrack/internal/config"
	"github.com/vaproh/uptrack/internal/gh"
)

func makeTarGz(t *testing.T, files map[string]string, modes map[string]int64) []byte {
	t.Helper()
	var buf bytes.Buffer
	g := gzip.NewWriter(&buf)
	tw := tar.NewWriter(g)
	for name, body := range files {
		mode := int64(0o644)
		if m, ok := modes[name]; ok {
			mode = m
		}
		err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(body))})
		if err != nil {
			t.Fatal(err)
		}
		_, err = tw.Write([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	g.Close()
	return buf.Bytes()
}

func TestExtractTarGz(t *testing.T) {
	data := makeTarGz(t,
		map[string]string{"tool": "#!/bin/sh\necho hi\n", "README.md": "hi\n"},
		map[string]int64{"tool": 0o755})
	dest := t.TempDir() + "/out"
	err := extractArchive(writeHelper(t, data), "gz", dest)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dest, "tool"))
	if err != nil || !strings.Contains(string(raw), "echo hi") {
		t.Fatalf("tool missing: %v", err)
	}
}

func TestExtractTraversalRejected(t *testing.T) {
	data := makeTarGz(t, map[string]string{"../../evil": "x\n"}, nil)
	err := extractArchive(writeHelper(t, data), "gz", t.TempDir()+"/out")
	if err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestExtractAbsoluteRejected(t *testing.T) {
	data := makeTarGz(t, map[string]string{"/abs/path": "x\n"}, nil)
	err := extractArchive(writeHelper(t, data), "gz", t.TempDir()+"/out")
	if err == nil {
		t.Fatal("expected absolute-path error")
	}
}

func writeHelper(t *testing.T, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "a.tar.gz")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const helloScript = "#!/bin/sh\necho hello\n"

// writeAr builds a minimal System V ar archive for .deb fixtures.
func writeAr(t *testing.T, members map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString("!<arch>\n")
	names := []string{"debian-binary", "control.tar.gz", "data.tar.gz"}
	for _, n := range names {
		data := members[n]
		hdr := fmt.Sprintf("%-16s%-12s%-6s%-6s%-8s%-10s`\n", n+"/", "0", "0", "0", "100644", fmt.Sprint(len(data)))
		buf.WriteString(hdr)
		buf.Write(data)
		if len(data)%2 == 1 {
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes()
}

func debFixture(t *testing.T) []byte {
	t.Helper()
	control := makeTarGz(t, map[string]string{"control": "Package: hello\n"}, nil)
	data := makeTarGz(t,
		map[string]string{
			"usr/bin/hello":                        helloScript,
			"usr/share/applications/hello.desktop": "[Desktop Entry]\nName=hello\nExec=/usr/bin/hello\n",
		},
		map[string]int64{"usr/bin/hello": 0o755})
	return writeAr(t, map[string][]byte{
		"debian-binary":  []byte("2.0\n"),
		"control.tar.gz": control,
		"data.tar.gz":    data,
	})
}

func serve(t *testing.T, data []byte) (*httptest.Server, string) {
	t.Helper()
	sum := fmt.Sprintf("sha256:%x", sha256.Sum256(data))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv, sum
}

func TestInstallDebEndToEnd(t *testing.T) {
	data := debFixture(t)
	srv, sum := serve(t, data)
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
	if !res.Verified {
		t.Fatal("expected verified install")
	}
	exe := filepath.Join(req.DestDir, res.ExeRel)
	raw, err := os.ReadFile(exe)
	if err != nil || string(raw) != helloScript {
		t.Fatalf("exe wrong: %v %q", err, raw)
	}
	link, err := os.Readlink(filepath.Join(req.BinDir, "hello"))
	if err != nil || link != exe {
		t.Fatalf("symlink wrong: %v %q", err, link)
	}
}

func TestInstallBinaryEndToEnd(t *testing.T) {
	data := []byte(helloScript)
	srv, sum := serve(t, data)
	root := t.TempDir()
	a := gh.Asset{Name: "hello-linux-amd64", Size: int64(len(data)), Digest: sum, DownloadURL: srv.URL + "/hello"}
	req := Request{
		App:     config.App{Name: "hello", Repo: "o/hello", Channel: "stable"},
		Sel:     asset.Selection{Asset: a, Kind: asset.KindBinary},
		Tag:     "v1.0",
		DestDir: filepath.Join(root, "opt", "hello"),
		BinDir:  filepath.Join(root, "bin"),
	}
	res, err := Install(context.Background(), req)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	fi, err := os.Stat(filepath.Join(req.DestDir, res.ExeRel))
	if err != nil || fi.Mode().Perm()&0o111 == 0 {
		t.Fatalf("exe not executable: %v", err)
	}
}

func TestInstallDigestMismatch(t *testing.T) {
	data := []byte(helloScript)
	srv, _ := serve(t, data)
	root := t.TempDir()
	a := gh.Asset{Name: "hello", Size: int64(len(data)), Digest: "sha256:0000000000000000000000000000000000000000000000000000000000000000", DownloadURL: srv.URL + "/x"}
	req := Request{
		App:     config.App{Name: "hello", Repo: "o/hello", Channel: "stable"},
		Sel:     asset.Selection{Asset: a, Kind: asset.KindBinary},
		Tag:     "v1.0",
		DestDir: filepath.Join(root, "opt", "hello"),
		BinDir:  filepath.Join(root, "bin"),
	}
	_, err := Install(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("expected checksum error, got %v", err)
	}
	if _, serr := os.Lstat(req.DestDir); !os.IsNotExist(serr) {
		t.Fatal("failed install must not leave a dest dir")
	}
}

func TestFindExeSingleFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "only"), []byte("x"), 0o755)
	got, err := findExe(dir, "")
	if err != nil || got != "only" {
		t.Fatalf("got %q %v", got, err)
	}
}

func TestFindExeAmbiguous(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a"), []byte("x"), 0o755)
	os.WriteFile(filepath.Join(dir, "b"), []byte("x"), 0o755)
	_, err := findExe(dir, "")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	got, err := findExe(dir, "b")
	if err != nil || got != "b" {
		t.Fatalf("hint failed: %q %v", got, err)
	}
}

func TestStageLivesWithDest(t *testing.T) {
	// Regression: staging in os.TempDir breaks the rename swap when
	// /tmp and the destination are different filesystems. Staging
	// must live next to the destination, and stale stages from
	// interrupted installs must be cleaned.
	data := []byte(helloScript)
	srv, sum := serve(t, data)
	root := t.TempDir()
	optDir := filepath.Join(root, "opt")
	if err := os.MkdirAll(optDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(optDir, ".uptrack-stage-12345")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	var calls []int64
	a := gh.Asset{Name: "hello", Size: int64(len(data)), Digest: sum, DownloadURL: srv.URL + "/x"}
	req := Request{
		App:      config.App{Name: "hello", Repo: "o/hello", Channel: "stable"},
		Sel:      asset.Selection{Asset: a, Kind: asset.KindBinary},
		Tag:      "v1.0",
		DestDir:  filepath.Join(optDir, "hello"),
		BinDir:   filepath.Join(root, "bin"),
		Progress: func(done, total int64) { calls = append(calls, done) },
	}
	_, err := Install(context.Background(), req)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, serr := os.Lstat(stale); !os.IsNotExist(serr) {
		t.Fatal("stale staging dir was not cleaned")
	}
	entries, err := os.ReadDir(optDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".uptrack-stage-") {
			t.Fatalf("staging dir leaked: %s", e.Name())
		}
	}
	if len(calls) == 0 {
		t.Fatal("progress callback never fired")
	}
	if calls[len(calls)-1] != int64(len(data)) {
		t.Fatalf("final progress %d, want %d", calls[len(calls)-1], len(data))
	}
	for i := 1; i < len(calls); i++ {
		if calls[i] < calls[i-1] {
			t.Fatal("progress must not go backwards")
		}
	}
}
