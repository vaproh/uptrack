// Package install downloads a release asset, verifies it, extracts
// it when needed, and places it under the install destination with
// a symlink in the bin dir. The old install stays until the new one
// is fully staged, so a failed install never breaks a working one.
package install

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vaproh/uptrack/internal/asset"
	"github.com/vaproh/uptrack/internal/config"
	"github.com/vaproh/uptrack/internal/gh"
)

// Request describes one installation.
type Request struct {
	App     config.App
	Sel     asset.Selection
	Tag     string
	DestDir string
	BinDir  string
	Client  *http.Client
}

// Result reports what was installed.
type Result struct {
	ExeRel   string // executable path relative to DestDir
	Verified bool   // checksum matched a published digest
}

// Install runs the full pipeline: download, verify, extract, place.
func Install(ctx context.Context, req Request) (Result, error) {
	client := req.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Minute}
	}
	tmp, err := os.CreateTemp("", "uptrack-dl-*")
	if err != nil {
		return Result{}, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	n, err := download(ctx, client, req.Sel.Asset, tmp)
	if err != nil {
		tmp.Close()
		return Result{}, err
	}
	err = tmp.Close()
	if err != nil {
		return Result{}, err
	}
	if req.Sel.Asset.Size > 0 && n != req.Sel.Asset.Size {
		return Result{}, fmt.Errorf("size mismatch: got %d bytes, release says %d", n, req.Sel.Asset.Size)
	}

	verified := false
	if req.Sel.Asset.Digest != "" {
		err = VerifyDigest(tmpName, req.Sel.Asset.Digest)
		if err != nil {
			return Result{}, err
		}
		verified = true
	}

	staged, exeRel, err := stage(req, tmpName)
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(staged)

	err = swapIn(req.DestDir, staged)
	if err != nil {
		return Result{}, err
	}
	exeAbs := filepath.Join(req.DestDir, exeRel)
	err = os.Chmod(exeAbs, 0o755)
	if err != nil {
		return Result{}, fmt.Errorf("chmod %s: %w", exeAbs, err)
	}
	err = linkBin(req.BinDir, req.App.Name, exeAbs)
	if err != nil {
		return Result{}, err
	}
	warnMissingLibs(exeAbs)
	return Result{ExeRel: exeRel, Verified: verified}, nil
}

// stage extracts the download into a fresh temp dir and locates the
// executable. It returns the staging dir and the exe's relative path.
func stage(req Request, file string) (string, string, error) {
	kind := req.Sel.Kind
	if req.App.Install != nil && req.App.Install.Type != "" {
		kind = asset.Kind(req.App.Install.Type)
	}
	staged, err := os.MkdirTemp("", "uptrack-stage-*")
	if err != nil {
		return "", "", err
	}
	exeHint := ""
	if req.App.Install != nil {
		exeHint = req.App.Install.Exe
	}
	switch kind {
	case asset.KindBinary, asset.KindAppImage:
		name := req.App.Name
		err = copyFile(file, filepath.Join(staged, name))
		if err != nil {
			os.RemoveAll(staged)
			return "", "", err
		}
		if exeHint != "" && exeHint != name {
			os.RemoveAll(staged)
			return "", "", fmt.Errorf("exe hint %q does not apply to a single-file install", exeHint)
		}
		return staged, name, nil
	case asset.KindArchive:
		err = extractArchive(file, req.Sel.Compression, staged)
		if err != nil {
			os.RemoveAll(staged)
			return "", "", err
		}
		exeRel, err := findExe(staged, exeHint)
		if err != nil {
			os.RemoveAll(staged)
			return "", "", err
		}
		return staged, exeRel, nil
	case asset.KindDeb:
		err = extractDeb(file, staged)
		if err != nil {
			os.RemoveAll(staged)
			return "", "", err
		}
		exeRel, err := findDebExe(staged, exeHint)
		if err != nil {
			os.RemoveAll(staged)
			return "", "", err
		}
		return staged, exeRel, nil
	default:
		os.RemoveAll(staged)
		return "", "", fmt.Errorf("unknown install type %q", kind)
	}
}

// swapIn moves staged into place. The previous install is kept aside
// until the new tree is renamed over it, then removed.
func swapIn(dest, staged string) error {
	err := os.MkdirAll(filepath.Dir(dest), 0o755)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(dest); os.IsNotExist(err) {
		return os.Rename(staged, dest)
	}
	old := dest + ".old"
	os.RemoveAll(old)
	err = os.Rename(dest, old)
	if err != nil {
		return fmt.Errorf("move old install aside: %w", err)
	}
	err = os.Rename(staged, dest)
	if err != nil {
		_ = os.Rename(old, dest)
		return fmt.Errorf("move new install in: %w", err)
	}
	os.RemoveAll(old)
	return nil
}

func linkBin(binDir, name, exeAbs string) error {
	err := os.MkdirAll(binDir, 0o755)
	if err != nil {
		return err
	}
	link := filepath.Join(binDir, name)
	if _, err := os.Lstat(link); err == nil {
		err = os.Remove(link)
		if err != nil {
			return fmt.Errorf("remove old symlink: %w", err)
		}
	}
	err = os.Symlink(exeAbs, link)
	if err != nil {
		return fmt.Errorf("symlink %s: %w", link, err)
	}
	return nil
}

func download(ctx context.Context, client *http.Client, a gh.Asset, w io.Writer) (int64, error) {
	return Download(ctx, client, a, w)
}

// Download fetches an asset URL into w, streaming.
func Download(ctx context.Context, client *http.Client, a gh.Asset, w io.Writer) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.DownloadURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/octet-stream")
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download: unexpected status %s", resp.Status)
	}
	n, err := io.Copy(w, resp.Body)
	if err != nil {
		return n, fmt.Errorf("download: %w", err)
	}
	return n, nil
}

// VerifyDigest checks "algo:hex" digests. Only sha256 and sha512 are
// supported; anything else is an explicit error, not a silent pass.
func VerifyDigest(path, digest string) error {
	algo, hexSum, ok := strings.Cut(digest, ":")
	if !ok {
		return fmt.Errorf("unrecognized digest format %q", digest)
	}
	var h hash.Hash
	switch strings.ToLower(algo) {
	case "sha256":
		h = sha256.New()
	case "sha512":
		h = sha512.New()
	default:
		return fmt.Errorf("unsupported digest algorithm %q", algo)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(h, f)
	if err != nil {
		return err
	}
	want, err := hex.DecodeString(strings.TrimSpace(hexSum))
	if err != nil {
		return fmt.Errorf("bad digest hex: %w", err)
	}
	if !equalFold(h.Sum(nil), want) {
		return fmt.Errorf("checksum mismatch: download does not match the published digest")
	}
	return nil
}

func equalFold(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

// InPath reports whether dir is on PATH.
func InPath(dir string) bool {
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if p == dir {
			return true
		}
	}
	return false
}
