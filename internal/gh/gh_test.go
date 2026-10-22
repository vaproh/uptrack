package gh

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const cannedPage1 = `[
  {"tag_name":"v1.19.4-beta.1","name":"beta","body":"","prerelease":true,"draft":false,
   "published_at":"2026-10-17T10:00:00Z","assets":[]},
  {"tag_name":"v1.19.3","name":"r","body":"notes","prerelease":false,"draft":false,
   "published_at":"2026-10-16T10:00:00Z",
   "assets":[
     {"name":"opencode-linux-x64.tar.gz","size":60716380,"digest":"sha256:abc123",
      "browser_download_url":"https://example.com/a.tar.gz"},
     {"name":"latest-linux.yml","size":584,"digest":"sha256:def456",
      "browser_download_url":"https://example.com/latest-linux.yml"}
   ]},
  {"tag_name":"v1.19.2","name":"old","body":"","prerelease":false,"draft":true,
   "published_at":"2026-10-10T10:00:00Z","assets":[]}
]`

func TestListAndSelect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); !strings.Contains(got, "application/vnd.github+json") {
			t.Errorf("missing accept header: %q", got)
		}
		fmt.Fprint(w, cannedPage1)
	}))
	defer srv.Close()

	c := New(srv.URL, "")
	rels, err := c.ListReleases(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	// Draft v1.19.2 must be skipped; order is newest first.
	if len(rels) != 2 {
		t.Fatalf("got %d releases, want 2", len(rels))
	}
	rel := LatestRelease(rels, "stable")
	if rel == nil || rel.TagName != "v1.19.3" {
		t.Fatalf("stable latest: %+v", rel)
	}
	if len(rel.Assets) != 2 || rel.Assets[0].Digest != "sha256:abc123" {
		t.Fatalf("assets not parsed: %+v", rel.Assets)
	}
	rel = LatestRelease(rels, "prerelease")
	if rel == nil || rel.TagName != "v1.19.4-beta.1" {
		t.Fatalf("prerelease latest: %+v", rel)
	}
	if LatestRelease(rels, "stable") == nil {
		t.Fatal("stable should exist")
	}
	got := FindRelease(rels, "v1.19.3")
	if got == nil {
		t.Fatal("find by tag failed")
	}
	if FindRelease(rels, "v9.9.9") != nil {
		t.Fatal("find should return nil for unknown tag")
	}
}

func TestPagination(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.RawQuery, "page=2") {
			fmt.Fprint(w, `[{"tag_name":"v1.0.0","prerelease":false,"draft":false,"published_at":"2026-01-01T00:00:00Z","assets":[]}]`)
			return
		}
		w.Header().Set("Link", `<`+srv.URL+`/x?page=2>; rel="next"`)
		fmt.Fprint(w, `[{"tag_name":"v2.0.0","prerelease":false,"draft":false,"published_at":"2026-02-01T00:00:00Z","assets":[]}]`)
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	rels, err := c.ListReleases(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rels) != 2 || rels[0].TagName != "v2.0.0" || rels[1].TagName != "v1.0.0" {
		t.Fatalf("pagination failed: %+v", rels)
	}
}

func TestParseRepo(t *testing.T) {
	cases := []struct {
		in        string
		owner     string
		repo      string
		wantError bool
	}{
		{"anomalyco/opencode", "anomalyco", "opencode", false},
		{"https://github.com/anomalyco/opencode", "anomalyco", "opencode", false},
		{"https://github.com/anomalyco/opencode/", "anomalyco", "opencode", false},
		{"https://github.com/anomalyco/opencode.git", "anomalyco", "opencode", false},
		{"justone", "", "", true},
		{"a/b/c", "", "", true},
		{"", "", "", true},
	}
	for _, c := range cases {
		o, r, err := ParseRepo(c.in)
		if c.wantError && err == nil {
			t.Errorf("ParseRepo(%q): expected error", c.in)
		}
		if !c.wantError && (err != nil || o != c.owner || r != c.repo) {
			t.Errorf("ParseRepo(%q) = %q %q %v", c.in, o, r, err)
		}
	}
}

func TestNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	}))
	defer srv.Close()
	c := New(srv.URL, "")
	_, err := c.ListReleases(context.Background(), "o", "nope")
	if err == nil {
		t.Fatal("expected not-found error")
	}
}
