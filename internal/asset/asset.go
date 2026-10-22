// Package asset picks the right file from a release. Selection runs
// on every release, since upstream renames assets between versions.
// Filters only ever reject; the winner is the surviving candidate
// with the best (demotion, format) rank. Ties are reported, never
// guessed.
package asset

import (
	"fmt"
	"strings"

	"github.com/vaproh/uptrack/internal/config"
	"github.com/vaproh/uptrack/internal/gh"
	"github.com/vaproh/uptrack/internal/platform"
)

// Kind is how the asset installs.
type Kind string

const (
	KindBinary   Kind = "binary"
	KindArchive  Kind = "archive"
	KindAppImage Kind = "appimage"
	KindDeb      Kind = "deb"
)

// Selection is the chosen asset plus how to handle it.
type Selection struct {
	Asset       gh.Asset
	Kind        Kind
	Compression string // gz, bz2, xz, zst, zip, none; archives only
}

// Line is one row of the selection log, used by `info`.
type Line struct {
	Asset    string
	Decision string // keep, demote, reject
	Reason   string
}

// AmbiguousError lists candidates that tied for first place.
type AmbiguousError struct {
	Candidates []gh.Asset
}

func (e *AmbiguousError) Error() string {
	names := make([]string, len(e.Candidates))
	for i, a := range e.Candidates {
		names[i] = a.Name
	}
	return "asset is ambiguous, candidates: " + strings.Join(names, ", ")
}

type scored struct {
	asset gh.Asset
	kind  Kind
	comp  string
	drank int // demotion rank: 0 normal, 1 baseline or gui
	frank int // format rank: binary 0, archive 1, appimage 2, deb 3
}

// Select returns the best asset for host, honoring rule when set.
func Select(assets []gh.Asset, rule *config.AssetRule, h platform.Host) (Selection, []Line, error) {
	var log []Line
	var kept []scored
	addLog := func(a gh.Asset, d, r string) {
		log = append(log, Line{Asset: a.Name, Decision: d, Reason: r})
	}

	for _, a := range assets {
		kind, comp, ok := detectFormat(a.Name)
		if !ok {
			addLog(a, "reject", "not an installable format")
			continue
		}
		if rule != nil && rule.Match != "" {
			matched, err := matchGlob(rule.Match, a.Name)
			if err != nil || !matched {
				addLog(a, "reject", "does not match rule")
				continue
			}
		}
		if rule != nil && excluded(rule.Exclude, a.Name) {
			addLog(a, "reject", "matches exclude pattern")
			continue
		}
		toks := tokens(a.Name)
		if os := detectOS(toks); os != "" && os != h.OS {
			addLog(a, "reject", "built for "+os)
			continue
		}
		if arch := detectArch(toks); arch != "" && arch != h.Arch {
			addLog(a, "reject", "built for "+arch)
			continue
		}
		libc := detectLibc(toks)
		if libc == "musl" && h.Libc == "glibc" {
			addLog(a, "reject", "musl build on a glibc host")
			continue
		}
		if (libc == "gnu" || libc == "glibc") && h.Libc == "musl" {
			addLog(a, "reject", "glibc build on a musl host")
			continue
		}
		drank := 0
		reason := "matches this system"
		if hasToken(toks, "baseline") {
			drank = 1
			reason = "matches, but built for older CPUs"
		} else if isGUI(toks) {
			drank = 1
			reason = "matches, but looks like a GUI build"
		}
		decision := "keep"
		if drank > 0 {
			decision = "demote"
		}
		addLog(a, decision, reason)
		kept = append(kept, scored{asset: a, kind: kind, comp: comp, drank: drank, frank: formatRank(kind)})
	}

	if len(kept) == 0 {
		return Selection{}, log, fmt.Errorf("no asset matches this system (%s/%s/%s)", h.OS, h.Arch, h.Libc)
	}
	best := kept[0]
	tied := []gh.Asset{best.asset}
	for _, s := range kept[1:] {
		switch {
		case s.drank < best.drank || (s.drank == best.drank && s.frank < best.frank):
			best = s
			tied = []gh.Asset{s.asset}
		case s.drank == best.drank && s.frank == best.frank:
			tied = append(tied, s.asset)
		}
	}
	if len(tied) > 1 {
		return Selection{}, log, &AmbiguousError{Candidates: tied}
	}
	return Selection{Asset: best.asset, Kind: best.kind, Compression: best.comp}, log, nil
}

// KindOf reports the install kind for a filename.
func KindOf(name string) (Kind, string, bool) {
	return detectFormat(name)
}

func formatRank(k Kind) int {
	switch k {
	case KindBinary:
		return 0
	case KindArchive:
		return 1
	case KindAppImage:
		return 2
	default:
		return 3
	}
}
