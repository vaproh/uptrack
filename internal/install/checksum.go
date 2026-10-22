package install

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/vaproh/uptrack/internal/gh"
)

var bsdSum = regexp.MustCompile(`^[A-Za-z0-9_]+\s+\(([^)]+)\)\s*=\s*([0-9a-fA-F]{64})\s*$`)

// resolveChecksum finds a checksum for assetName among sibling assets
// when the asset itself carries no digest. Per-file sidecars
// (<name>.sha256) win over aggregate files (SHA256SUMS et al).
// Anything failing to download or parse is skipped, never fatal.
// It returns a "sha256:<hex>" digest and the file that proved it.
func resolveChecksum(ctx context.Context, client *http.Client, assets []gh.Asset, assetName string) (string, string) {
	lower := strings.ToLower(assetName)
	var perFile, aggregate []gh.Asset
	for _, a := range assets {
		n := strings.ToLower(a.Name)
		switch {
		case n == lower+".sha256" || n == lower+".sha256sum" || n == lower+".sha256sums":
			perFile = append(perFile, a)
		case n == "sha256sums" || n == "sha256sums.txt" || n == "checksums" || n == "checksums.txt":
			aggregate = append(aggregate, a)
		}
	}
	for _, a := range append(perFile, aggregate...) {
		hex, ok := checksumFor(ctx, client, a, assetName)
		if ok {
			return "sha256:" + hex, a.Name
		}
	}
	return "", ""
}

// checksumFor downloads one checksum file and extracts the hex for
// assetName. It understands "<hex>  <file>" and "ALGO (file) = <hex>".
func checksumFor(ctx context.Context, client *http.Client, sums gh.Asset, assetName string) (string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sums.DownloadURL, nil)
	if err != nil {
		return "", false
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if hex, ok := checksumLine(line, assetName); ok {
			return hex, true
		}
	}
	return "", false
}

func checksumLine(line, assetName string) (string, bool) {
	if m := bsdSum.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
		if base(m[1]) == assetName && isHex(m[2]) {
			return strings.ToLower(m[2]), true
		}
		return "", false
	}
	fields := strings.Fields(line)
	if len(fields) < 2 || !isHex(fields[0]) {
		return "", false
	}
	last := strings.TrimPrefix(fields[len(fields)-1], "*")
	if base(last) == assetName {
		return strings.ToLower(fields[0]), true
	}
	return "", false
}

func base(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func isHex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return false
		}
	}
	return true
}
