package install

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// openDecompressed returns the archive stream for a compression.
// xz and zstd shell out to the system binaries; everything else is
// stdlib. shortcut: this keeps the binary dependency-free.
func openDecompressed(path, comp string) (io.ReadCloser, error) {
	switch comp {
	case "gz":
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		g, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		return &closeBoth{g, f}, nil
	case "bz2":
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return &closeBoth{io.NopCloser(bzip2.NewReader(f)), f}, nil
	case "xz", "zst":
		bin := comp
		if comp == "zst" {
			bin = "zstd"
		}
		if _, err := exec.LookPath(bin); err != nil {
			return nil, fmt.Errorf("%s is required to extract this archive but was not found on PATH", bin)
		}
		cmd := exec.Command(bin, "-dc", path)
		out, err := cmd.StdoutPipe()
		if err != nil {
			return nil, err
		}
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("start %s: %w", bin, err)
		}
		return &cmdCloser{out, cmd}, nil
	case "none":
		return os.Open(path)
	default:
		return nil, fmt.Errorf("unsupported compression %q", comp)
	}
}

type closeBoth struct {
	io.ReadCloser
	second io.Closer
}

func (c *closeBoth) Close() error {
	err1 := c.ReadCloser.Close()
	err2 := c.second.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

type cmdCloser struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (c *cmdCloser) Close() error {
	err1 := c.ReadCloser.Close()
	err2 := c.cmd.Wait()
	if err1 != nil {
		return err1
	}
	return err2
}

// extractArchive unpacks a tar.* or zip file into dest.
func extractArchive(path, comp, dest string) error {
	if comp == "zip" {
		return extractZip(path, dest)
	}
	stream, err := openDecompressed(path, comp)
	if err != nil {
		return err
	}
	defer stream.Close()
	return extractTar(tar.NewReader(stream), dest)
}

// safeJoin resolves name inside dest, rejecting absolute paths,
// parent-directory escapes and anything that is not a plain file,
// directory or in-tree symlink. shortcut: hardlinks, devices and
// fifos are skipped rather than supported.
func safeJoin(dest, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", fmt.Errorf("archive entry %q is absolute", name)
	}
	clean := filepath.Clean(name)
	if clean == "." {
		return "", nil
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("archive entry %q escapes its directory", name)
	}
	return filepath.Join(dest, clean), nil
}

type symlinkJob struct {
	path   string
	target string
}

func extractTar(tr *tar.Reader, dest string) error {
	var links []symlinkJob
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read archive: %w", err)
		}
		target, err := safeJoin(dest, hdr.Name)
		if err != nil {
			return err
		}
		if target == "" {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(target, hdr.FileInfo().Mode()|0o755)
			if err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			err = os.MkdirAll(filepath.Dir(target), 0o755)
			if err != nil {
				return err
			}
			err = writeFile(tr, target, hdr.FileInfo().Mode())
			if err != nil {
				return err
			}
		case tar.TypeSymlink:
			links = append(links, symlinkJob{target, hdr.Linkname})
		default:
			// shortcut: devices, fifos and hardlinks are skipped.
			continue
		}
	}
	for _, l := range links {
		resolved := l.target
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(filepath.Dir(l.path), resolved)
		}
		rel, err := filepath.Rel(dest, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
			continue // shortcut: links escaping the install dir are dropped
		}
		os.MkdirAll(filepath.Dir(l.path), 0o755)
		os.Remove(l.path)
		_ = os.Symlink(l.target, l.path)
	}
	return nil
}

func extractZip(path, dest string) error {
	z, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("read archive: %w", err)
	}
	defer z.Close()
	var links []symlinkJob
	for _, f := range z.File {
		target, err := safeJoin(dest, f.Name)
		if err != nil {
			return err
		}
		if target == "" {
			continue
		}
		if f.FileInfo().IsDir() {
			err = os.MkdirAll(target, 0o755)
			if err != nil {
				return err
			}
			continue
		}
		if f.Mode()&os.ModeSymlink != 0 {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			raw, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return err
			}
			links = append(links, symlinkJob{target, string(raw)})
			continue
		}
		err = os.MkdirAll(filepath.Dir(target), 0o755)
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		err = writeFile(rc, target, f.Mode())
		rc.Close()
		if err != nil {
			return err
		}
	}
	for _, l := range links {
		resolved := l.target
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(filepath.Dir(l.path), resolved)
		}
		rel, err := filepath.Rel(dest, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
			continue
		}
		os.MkdirAll(filepath.Dir(l.path), 0o755)
		os.Remove(l.path)
		_ = os.Symlink(l.target, l.path)
	}
	return nil
}

func writeFile(r io.Reader, target string, mode os.FileMode) error {
	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm()&0o777)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, r)
	if err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
