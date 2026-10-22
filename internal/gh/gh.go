// Package gh is a minimal GitHub releases client. Only the releases
// API is used; pagination follows Link headers.
package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Asset is one file attached to a release.
type Asset struct {
	Name        string
	Size        int64
	Digest      string // "sha256:<hex>" when GitHub provides it
	DownloadURL string
}

// Release is one GitHub release with its assets inline.
type Release struct {
	TagName     string
	Name        string
	Body        string
	Prerelease  bool
	Draft       bool
	PublishedAt time.Time
	Assets      []Asset
}

// Client talks to the GitHub releases API.
type Client struct {
	base   string
	token  string
	client *http.Client
}

// New builds a client. base is the API root and is only overridden
// in tests. UPTRACK_GITHUB_API overrides the default API root, which
// also enables GitHub Enterprise Server. An empty token means
// unauthenticated (60 req/hour).
func New(base, token string) *Client {
	if base == "" {
		base = getenv("UPTRACK_GITHUB_API")
	}
	if base == "" {
		base = "https://api.github.com"
	}
	return &Client{
		base:  strings.TrimSuffix(base, "/"),
		token: token,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// TokenFromEnv returns GITHUB_TOKEN when set.
func TokenFromEnv() string {
	return getenv("GITHUB_TOKEN")
}

// ParseRepo accepts "owner/repo" or a full github.com URL.
func ParseRepo(in string) (owner, repo string, err error) {
	in = strings.TrimSpace(strings.TrimSuffix(in, "/"))
	if strings.Contains(in, "://") {
		u, err := url.Parse(in)
		if err != nil {
			return "", "", fmt.Errorf("parse repo %q: %w", in, err)
		}
		in = strings.TrimPrefix(u.Path, "/")
	}
	parts := strings.Split(in, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("want owner/repo, got %q", in)
	}
	return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
}

type assetJSON struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	Digest             string `json:"digest"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type releaseJSON struct {
	TagName     string      `json:"tag_name"`
	Name        string      `json:"name"`
	Body        string      `json:"body"`
	Prerelease  bool        `json:"prerelease"`
	Draft       bool        `json:"draft"`
	PublishedAt time.Time   `json:"published_at"`
	Assets      []assetJSON `json:"assets"`
}

func toRelease(r releaseJSON) Release {
	rel := Release{
		TagName:     r.TagName,
		Name:        r.Name,
		Body:        r.Body,
		Prerelease:  r.Prerelease,
		Draft:       r.Draft,
		PublishedAt: r.PublishedAt,
	}
	for _, a := range r.Assets {
		rel.Assets = append(rel.Assets, Asset{
			Name:        a.Name,
			Size:        a.Size,
			Digest:      a.Digest,
			DownloadURL: a.BrowserDownloadURL,
		})
	}
	return rel
}

func (c *Client) get(ctx context.Context, url string, out any) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("github request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("repository or release not found")
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == 429 {
		return "", fmt.Errorf("github rate limit reached; set GITHUB_TOKEN")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github: unexpected status %s", resp.Status)
	}
	err = json.NewDecoder(resp.Body).Decode(out)
	if err != nil {
		return "", fmt.Errorf("decode github response: %w", err)
	}
	return resp.Header.Get("Link"), nil
}

// ListReleases returns releases newest first, skipping drafts.
func (c *Client) ListReleases(ctx context.Context, owner, repo string) ([]Release, error) {
	var out []Release
	next := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=100", c.base, owner, repo)
	for next != "" {
		var page []releaseJSON
		link, err := c.get(ctx, next, &page)
		if err != nil {
			return nil, err
		}
		for _, r := range page {
			if r.Draft {
				continue
			}
			out = append(out, toRelease(r))
		}
		next = nextPage(link)
	}
	return out, nil
}

// LatestRelease returns the newest eligible release for a channel.
// stable excludes prereleases; prerelease and any include them.
func LatestRelease(rels []Release, channel string) *Release {
	for i := range rels {
		if channel == "stable" && rels[i].Prerelease {
			continue
		}
		return &rels[i]
	}
	return nil
}

// FindRelease returns the release with the given tag, or nil.
func FindRelease(rels []Release, tag string) *Release {
	for i := range rels {
		if rels[i].TagName == tag {
			return &rels[i]
		}
	}
	return nil
}

// Latest fetches the single latest published release. It returns
// nil without error when the repo has no releases.
func (c *Client) Latest(ctx context.Context, owner, repo string) (*Release, error) {
	var r releaseJSON
	_, err := c.get(ctx, fmt.Sprintf("%s/repos/%s/%s/releases/latest", c.base, owner, repo), &r)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, err
	}
	rel := toRelease(r)
	return &rel, nil
}

// nextPage extracts the URL with rel="next" from a Link header.
func nextPage(link string) string {
	for _, part := range strings.Split(link, ",") {
		part = strings.TrimSpace(part)
		if !strings.HasSuffix(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start >= 0 && end > start {
			return part[start+1 : end]
		}
	}
	return ""
}
