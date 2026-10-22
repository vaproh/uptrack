package version

import "testing"

func TestCompare(t *testing.T) {
	cases := []struct {
		a    string
		b    string
		want int
	}{
		{"v1.19.2", "v1.19.3", -1},
		{"v1.19.3", "v1.19.2", 1},
		{"v1.19.3", "v1.19.3", 0},
		{"1.2.3", "v1.2.3", 0},
		{"v1.10.0", "v1.9.9", 1}, // numeric, not lexical
		{"v2.0", "v10.0", -1},
		{"v1.2.3-beta.1", "v1.2.3", -1}, // prerelease below release
		{"v1.2.3-alpha", "v1.2.3-beta", -1},
		{"2024.01.05", "2024.10.01", -1},  // date versions
		{"v1.19.3+build.1", "v1.19.3", 0}, // build metadata ignored
		{"V1.2.3", "v1.2.3", 0},
		{"1.2", "1.2.0", -1}, // shorter is older
	}
	for _, c := range cases {
		got := Compare(c.a, c.b)
		if (got < 0 && c.want < 0) || (got == 0 && c.want == 0) || (got > 0 && c.want > 0) {
			continue
		}
		t.Errorf("Compare(%q, %q) = %d, want sign %d", c.a, c.b, got, c.want)
	}
}
