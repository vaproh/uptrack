# uptrack — PRD

## What it is

A personal CLI that installs and updates standalone Linux apps from
GitHub releases. Target machines: Arch, Artix, Void, Gentoo — all
glibc x86_64 today, musl-aware for later.

## Non-goals

Generic tracker, not a catalogue. No curated recipes, no plugin system.
rpm, signatures, rollback command, non-GitHub sources, TUI, JSON
output, scheduled checks, and anything touching Upall are out.

## Config

`~/.config/uptrack/config.yaml`, XDG-aware, kept in dotfiles. Required
per app: `name`, `repo`, `channel` (stable | prerelease | any).
Everything else inferred, overridable per app:

```yaml
asset: {match: glob, exclude: [globs]}
install: {type: binary|archive|appimage|deb, dest, exe}
```

State (installed/latest versions) in `~/.local/state/uptrack/state.json`.
Installs to `~/.local/opt/<name>/`, symlinked from `~/.local/bin/`.

## Asset selection

Re-evaluated every release. Filter order: installable format, OS, arch,
libc (reject musl on glibc and vice versa), then demote `baseline` and
GUI builds, then prefer binary > archive > appimage > deb. Ties prompt
on a TTY and error otherwise. A stored rule matching nothing falls back
to automatic selection with a warning.

## Install pipeline

Temp download → size check → sha256 vs published digest (missing digest
= labeled unverified, mismatch = abort) → extract → stage → atomic
rename swap → chmod → symlink → record state. Rollback is
`install --version <older-tag>`.

Format handling: tar.gz/bz2 + zip from stdlib, xz/zst via system
binaries, `.deb` via Go-native `ar` parse of `data.tar.*` only (no
maintainer scripts, no root, no dependency resolution; `ldd` warns on
missing libs). `.deb` exe resolution: `.desktop` Exec line, then lone
binary in `usr/bin`. Archive exe resolution: hint, single root file,
single top dir, exactly one executable — else require `install.exe`.

## Interface

CLI only. Installs/upgrades confirm unless `-y`. Colors on TTY only,
`NO_COLOR` and `--no-color` respected. Nonzero exit on any failure;
partial failures in `check`/`sync` report per app and continue.
`uptrack self-update` replaces the uptrack binary itself from the
latest GitHub release, verified against `sha256sums.txt`.

## Known limits

- `.deb` needs its shared libraries from the system package manager.
- AppImages need FUSE.
- Auto-selection covers common naming; `asset.match` covers the rest.
- Unauthenticated API: 60 req/hour (`GITHUB_TOKEN` honored).
