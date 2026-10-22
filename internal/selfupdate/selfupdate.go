// Package selfupdate replaces the running uptrack binary with the
// latest release. It reuses the install package for download and
// verification; printing and confirmation stay in cli.
package selfupdate

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/vaproh/uptrack/internal/gh"
	"github.com/vaproh/uptrack/internal/install"
	"github.com/vaproh/uptrack/internal/version"
)

// Source of uptrack's own releases.
const (
	Owner = "vaproh"
	Repo  = "uptrack"
)

// Update fetches the latest uptrack release and, when it is newer
// than currentVersion, replaces the binary at currentPath. It returns
// the latest tag and whether an update was installed. "dev" builds
// always update.
func Update(ctx context.Context, client *gh.Client, currentPath, currentVersion string) (string, bool, error) {
	rel, err := client.Latest(ctx, Owner, Repo)
	if err != nil {
		return "", false, err
	}
	if rel == nil {
		return "", false, fmt.Errorf("no uptrack releases found")
	}
	if currentVersion != "dev" && version.Compare(currentVersion, rel.TagName) >= 0 {
		return rel.TagName, false, nil
	}
	if runtime.GOOS != "linux" {
		return "", false, fmt.Errorf("self-update supports linux only")
	}
	suffix, ok := map[string]string{"amd64": "x86_64", "arm64": "aarch64"}[runtime.GOARCH]
	if !ok {
		return "", false, fmt.Errorf("self-update supports x86_64 and aarch64 only")
	}
	var bin, sums *gh.Asset
	for i := range rel.Assets {
		a := &rel.Assets[i]
		switch a.Name {
		case "uptrack-linux-" + suffix:
			bin = a
		case "sha256sums.txt":
			sums = a
		}
	}
	if bin == nil {
		return "", false, fmt.Errorf("release %s has no binary for this system", rel.TagName)
	}
	if sums == nil {
		return "", false, fmt.Errorf("release %s has no sha256sums.txt", rel.TagName)
	}
	want, err := wantDigest(ctx, sums, bin.Name)
	if err != nil {
		return "", false, err
	}

	tmp, err := os.CreateTemp("", "uptrack-self-*")
	if err != nil {
		return "", false, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	httpClient := &http.Client{Timeout: 30 * time.Minute}
	_, err = install.Download(ctx, httpClient, *bin, tmp)
	if err != nil {
		tmp.Close()
		return "", false, err
	}
	err = tmp.Close()
	if err != nil {
		return "", false, err
	}
	err = install.VerifyDigest(tmpName, "sha256:"+want)
	if err != nil {
		return "", false, err
	}
	err = os.Chmod(tmpName, 0o755)
	if err != nil {
		return "", false, err
	}
	err = os.Rename(tmpName, currentPath)
	if err != nil {
		return "", false, fmt.Errorf("replace binary: %w", err)
	}
	return rel.TagName, true, nil
}

// wantDigest downloads the sums file and returns the hex for asset.
func wantDigest(ctx context.Context, sums *gh.Asset, asset string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sums.DownloadURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download checksums: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download checksums: unexpected status %s", resp.Status)
	}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(buf.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no checksum for %s in sha256sums.txt", asset)
}
