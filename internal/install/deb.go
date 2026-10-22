package install

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// extractDeb unpacks the data.tar.* payload of a .deb into dest.
// Maintainer scripts and the dpkg database are never touched:
// a .deb here is just an ar container holding a file tree.
func extractDeb(path, dest string) error {
	members, err := readAr(path)
	if err != nil {
		return err
	}
	var data []byte
	dataName := ""
	for _, m := range members {
		if strings.HasPrefix(m.name, "data.tar") {
			data = m.data
			dataName = m.name
		}
	}
	if data == nil {
		return fmt.Errorf("not a deb: no data.tar.* member")
	}
	comp := "none"
	switch {
	case strings.HasSuffix(dataName, ".gz"):
		comp = "gz"
	case strings.HasSuffix(dataName, ".bz2"):
		comp = "bz2"
	case strings.HasSuffix(dataName, ".xz"):
		comp = "xz"
	case strings.HasSuffix(dataName, ".zst"):
		comp = "zst"
	}
	tmp, err := writeTemp(data)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	stream, err := openDecompressed(tmp, comp)
	if err != nil {
		return err
	}
	defer stream.Close()
	return extractTar(tar.NewReader(stream), dest)
}

type arMember struct {
	name string
	data []byte
}

// readAr parses System V archives (the GNU ar dialect .debs use).
// BSD long-name entries are rejected; .debs do not use them.
func readAr(path string) ([]arMember, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	magic := make([]byte, 8)
	_, err = io.ReadFull(f, magic)
	if err != nil {
		return nil, fmt.Errorf("read ar header: %w", err)
	}
	if string(magic) != "!<arch>\n" {
		return nil, fmt.Errorf("not a deb: bad ar magic")
	}
	var out []arMember
	for {
		hdr := make([]byte, 60)
		_, err = io.ReadFull(f, hdr)
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read ar member: %w", err)
		}
		name := strings.TrimRight(string(hdr[0:16]), " ")
		name = strings.TrimSuffix(name, "/")
		if strings.HasPrefix(name, "#1/") {
			return nil, fmt.Errorf("unsupported BSD long filename in ar archive")
		}
		size, err := strconv.ParseInt(strings.TrimSpace(string(hdr[48:58])), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bad ar member size: %w", err)
		}
		if hdr[58] != '`' || hdr[59] != '\n' {
			return nil, fmt.Errorf("bad ar member terminator")
		}
		data := make([]byte, size)
		_, err = io.ReadFull(f, data)
		if err != nil {
			return nil, fmt.Errorf("read ar member %q: %w", name, err)
		}
		out = append(out, arMember{name: name, data: data})
		if size%2 == 1 {
			var pad [1]byte
			_, _ = io.ReadFull(f, pad[:])
		}
	}
	return out, nil
}

func writeTemp(data []byte) (string, error) {
	f, err := os.CreateTemp("", "uptrack-deb-*")
	if err != nil {
		return "", err
	}
	name := f.Name()
	_, err = f.Write(data)
	if err != nil {
		f.Close()
		os.Remove(name)
		return "", err
	}
	err = f.Close()
	if err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}
