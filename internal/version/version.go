// Package version compares release tags. Tags are normalized
// (leading v, build metadata) and compared segment by segment:
// numeric segments compare numerically, others lexically, and a
// plain release sorts above any prerelease of the same numbers.
package version

import (
	"strconv"
	"strings"
)

// Compare returns -1, 0 or +1 for a < b, a == b, a > b.
func Compare(a, b string) int {
	na, pa := split(normalize(a))
	nb, pb := split(normalize(b))
	for i := 0; i < len(na) && i < len(nb); i++ {
		if na[i] == nb[i] {
			continue
		}
		ai, aerr := strconv.Atoi(na[i])
		bi, berr := strconv.Atoi(nb[i])
		if aerr == nil && berr == nil {
			if ai < bi {
				return -1
			}
			return 1
		}
		if na[i] < nb[i] {
			return -1
		}
		return 1
	}
	switch {
	case len(na) < len(nb):
		return -1
	case len(na) > len(nb):
		return 1
	}
	switch {
	case pa == "" && pb != "":
		return 1
	case pa != "" && pb == "":
		return -1
	case pa < pb:
		return -1
	case pa > pb:
		return 1
	}
	return 0
}

func normalize(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if i := strings.Index(v, "+"); i >= 0 {
		v = v[:i]
	}
	return v
}

func split(v string) (nums []string, pre string) {
	if i := strings.Index(v, "-"); i >= 0 {
		pre = v[i+1:]
		v = v[:i]
	}
	return strings.Split(v, "."), pre
}
