package install

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vaproh/uptrack/internal/asset"
	"github.com/vaproh/uptrack/internal/config"
	"github.com/vaproh/uptrack/internal/gh"
)

func TestChecksumLine(t *testing.T) {
	hex := strings.Repeat("ab", 32)
	cases := []struct {
		line string
		name string
		want string
	}{
		{hex + "  tool-linux-amd64.tar.gz", "tool-linux-amd64.tar.gz", hex},
		{hex + " *tool-linux-amd64.tar.gz", "tool-linux-amd64.tar.gz", hex},
		{"SHA256 (tool.tar.gz) = " + hex, "tool.tar.gz", hex},
		{hex + "  other.tar.gz", "tool.tar.gz", ""},
		{"not a checksum line", "tool.tar.gz", ""},
		{"xyz  tool.tar.gz", "tool.tar.gz", ""},
	}
	for _, c := range cases {
		got, ok := checksumLine(c.line, c.name)
		if c.want == "" && ok {
			t.Errorf("%q: unexpected match %q", c.line, got)
		}
		if c.want != "" && (!ok || got != c.want) {
			t.Errorf("%q: got %q,%v want %q", c.line, got, ok, c.want)
		}
	}
}

func serveRelease(t *testing.T, files map[string][]byte) (*httptest.Server, []gh.Asset) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if data, ok := files[name]; ok {
			w.Write(data)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	var assets []gh.Asset
	for name, data := range files {
		assets = append(assets, gh.Asset{
			Name:        name,
			Size:        int64(len(data)),
			DownloadURL: srv.URL + "/" + name,
		})
	}
	return srv, assets
}

func installNoDigest(t *testing.T, files map[string][]byte, assetName string, kind asset.Kind) (Result, error) {
	t.Helper()
	_, assets := serveRelease(t, files)
	var bin gh.Asset
	for _, a := range assets {
		if a.Name == assetName {
			bin = a
		}
	}
	root := t.TempDir()
	req := Request{
		App:     config.App{Name: "tool", Repo: "o/tool", Channel: "stable"},
		Sel:     asset.Selection{Asset: bin, Kind: kind},
		Tag:     "v1.0",
		DestDir: filepath.Join(root, "opt", "tool"),
		BinDir:  filepath.Join(root, "bin"),
		Assets:  assets,
	}
	return Install(context.Background(), req)
}

func TestChecksumAggregate(t *testing.T) {
	tool := []byte(helloScript)
	sum := fmt.Sprintf("%x", sha256.Sum256(tool))
	res, err := installNoDigest(t, map[string][]byte{
		"tool":       tool,
		"SHA256SUMS": []byte(sum + "  tool\n" + strings.Repeat("cd", 32) + "  other\n"),
	}, "tool", asset.KindBinary)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !res.Verified || res.VerifiedBy != "SHA256SUMS" {
		t.Fatalf("want verified via SHA256SUMS, got %+v", res)
	}
}

func TestChecksumSidecar(t *testing.T) {
	tool := []byte(helloScript)
	sum := fmt.Sprintf("%x", sha256.Sum256(tool))
	res, err := installNoDigest(t, map[string][]byte{
		"tool":        tool,
		"tool.sha256": []byte(sum + "  tool\n"),
	}, "tool", asset.KindBinary)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !res.Verified || res.VerifiedBy != "tool.sha256" {
		t.Fatalf("want verified via sidecar, got %+v", res)
	}
}

func TestChecksumMissing(t *testing.T) {
	tool := []byte(helloScript)
	res, err := installNoDigest(t, map[string][]byte{
		"tool":       tool,
		"SHA256SUMS": []byte(strings.Repeat("cd", 32) + "  other\n"),
	}, "tool", asset.KindBinary)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if res.Verified {
		t.Fatalf("want unverified, got %+v", res)
	}
}

func TestChecksumWrong(t *testing.T) {
	tool := []byte(helloScript)
	_, err := installNoDigest(t, map[string][]byte{
		"tool":       tool,
		"SHA256SUMS": []byte(strings.Repeat("cd", 32) + "  tool\n"),
	}, "tool", asset.KindBinary)
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("want checksum error, got %v", err)
	}
}
