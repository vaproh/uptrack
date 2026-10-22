package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeRelease serves one release with one linux tarball whose
// download URL points back at the test server.
func fakeRelease(t *testing.T, srvURL string, tool []byte) (string, string) {
	t.Helper()
	var buf bytes.Buffer
	g := gzip.NewWriter(&buf)
	tw := tar.NewWriter(g)
	err := tw.WriteHeader(&tar.Header{Name: "tool", Mode: 0o755, Size: int64(len(tool))})
	if err != nil {
		t.Fatal(err)
	}
	_, err = tw.Write(tool)
	if err != nil {
		t.Fatal(err)
	}
	tw.Close()
	g.Close()
	blob := buf.Bytes()
	sum := fmt.Sprintf("sha256:%x", sha256.Sum256(blob))
	return fmt.Sprintf(`[{
  "tag_name": "v2.0.0", "name": "r", "body": "notes", "prerelease": false, "draft": false,
  "published_at": "2026-01-01T00:00:00Z",
  "assets": [{
    "name": "tool-linux-amd64.tar.gz", "size": %d, "digest": %q,
    "browser_download_url": %q
  }]
}]`, len(blob), sum, srvURL+"/dl/tool.tar.gz"), string(blob)
}

func TestSmoke(t *testing.T) {
	tool := []byte("#!/bin/sh\necho smoke\n")
	var blob []byte
	var releases string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/releases") {
			fmt.Fprint(w, releases)
			return
		}
		w.Write(blob)
	}))
	defer srv.Close()
	releases, blobStr := fakeRelease(t, srv.URL, tool)
	blob = []byte(blobStr)

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("UPTRACK_GITHUB_API", srv.URL)

	run := func(args ...string) string {
		// Commands write to stdout directly, so capture the fd.
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		root := NewRoot()
		root.SetArgs(args)
		err := root.Execute()
		w.Close()
		os.Stdout = old
		var out bytes.Buffer
		_, _ = out.ReadFrom(r)
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out.String())
		}
		return out.String()
	}

	run("add", "o/tool")
	out := run("list")
	if !strings.Contains(out, "tool") || !strings.Contains(out, "v2.0.0") {
		t.Fatalf("list missing app: %s", out)
	}
	run("check")
	out = run("outdated")
	if !strings.Contains(out, "tool") {
		t.Fatalf("outdated should show uninstalled app as pending: %s", out)
	}
	run("install", "tool", "-y")
	link := filepath.Join(home, ".local", "bin", "tool")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("symlink missing: %v", err)
	}
	raw, err := os.ReadFile(target)
	if err != nil || string(raw) != string(tool) {
		t.Fatalf("installed content wrong: %v", err)
	}
	out = run("outdated")
	if !strings.Contains(out, "up to date") {
		t.Fatalf("should be up to date after install: %s", out)
	}
	run("remove", "tool", "--purge")
	if _, err := os.Lstat(filepath.Join(home, ".local", "opt", "tool")); !os.IsNotExist(err) {
		t.Fatal("purge should delete install dir")
	}
	_ = releases
}
