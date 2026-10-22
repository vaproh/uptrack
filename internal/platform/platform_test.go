package platform

import "testing"

func TestNormalizeArch(t *testing.T) {
	cases := []struct{ in, want string }{
		{"amd64", "amd64"},
		{"x86_64", "amd64"},
		{"x86-64", "amd64"},
		{"x64", "amd64"},
		{"arm64", "arm64"},
		{"aarch64", "arm64"},
		{"386", "386"},
		{"i686", "386"},
		{"armv7l", "arm"},
		{"mips", "mips"},
	}
	for _, c := range cases {
		got := NormalizeArch(c.in)
		if got != c.want {
			t.Errorf("NormalizeArch(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDetect(t *testing.T) {
	h := Detect()
	if h.OS == "" || h.Arch == "" {
		t.Fatalf("empty host: %+v", h)
	}
	if h.Libc != "glibc" && h.Libc != "musl" {
		t.Fatalf("bad libc: %q", h.Libc)
	}
	t.Logf("host: %+v", h)
}
