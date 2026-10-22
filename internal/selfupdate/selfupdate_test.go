package selfupdate

import (
	"context"

	"crypto/sha256"
	"fmt"
	"github.com/vaproh/uptrack/internal/gh"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func fakeServer(t *testing.T, tag string, blob []byte) *httptest.Server {
	t.Helper()
	sum := fmt.Sprintf("%x", sha256.Sum256(blob))
	sums := fmt.Sprintf("%s  uptrack-linux-x86_64\n", sum)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			fmt.Fprintf(w, `{"tag_name":%q,"prerelease":false,"draft":false,
				"published_at":"2026-01-01T00:00:00Z",
				"assets":[
				{"name":"uptrack-linux-x86_64","size":%d,"digest":"",
					"browser_download_url":%q},
				{"name":"sha256sums.txt","size":%d,"digest":"",
					"browser_download_url":%q}]}`,
				tag, len(blob), srv.URL+"/dl/bin",
				len(sums), srv.URL+"/dl/sums")
		case r.URL.Path == "/dl/bin":
			w.Write(blob)
		case r.URL.Path == "/dl/sums":
			fmt.Fprint(w, sums)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestUpdateInstalls(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("self-update fixture targets linux/amd64")
	}
	srv := fakeServer(t, "v0.2.0", []byte("new-binary"))
	exe := filepath.Join(t.TempDir(), "uptrack")
	if err := os.WriteFile(exe, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	client := gh.New(srv.URL, "")
	tag, updated, err := Update(context.Background(), client, exe, "v0.1.0", nil)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !updated || tag != "v0.2.0" {
		t.Fatalf("got %q %v, want v0.2.0 true", tag, updated)
	}
	raw, err := os.ReadFile(exe)
	if err != nil || string(raw) != "new-binary" {
		t.Fatalf("binary not replaced: %v %q", err, raw)
	}
	fi, err := os.Stat(exe)
	if err != nil || fi.Mode().Perm()&0o111 == 0 {
		t.Fatalf("not executable: %v", err)
	}
}

func TestUpdateCurrent(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("self-update fixture targets linux/amd64")
	}
	srv := fakeServer(t, "v0.2.0", []byte("new-binary"))
	exe := filepath.Join(t.TempDir(), "uptrack")
	if err := os.WriteFile(exe, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	client := gh.New(srv.URL, "")
	tag, updated, err := Update(context.Background(), client, exe, "v0.2.0", nil)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated {
		t.Fatal("should not update when current")
	}
	if tag != "v0.2.0" {
		t.Fatalf("got tag %q", tag)
	}
	raw, _ := os.ReadFile(exe)
	if string(raw) != "old-binary" {
		t.Fatal("binary must be untouched")
	}
}
