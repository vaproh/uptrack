# uptrack

[![ci](https://github.com/vaproh/uptrack/actions/workflows/ci.yml/badge.svg)](https://github.com/vaproh/uptrack/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/vaproh/uptrack)](https://github.com/vaproh/uptrack/releases)
![go](https://img.shields.io/badge/go-1.27-00ADD8)
![license](https://img.shields.io/badge/license-MIT-green)
![platform](https://img.shields.io/badge/platform-linux-lightgrey)

Track and install standalone Linux apps from GitHub releases. No root,
no package database, no lock-in: one YAML file lists your apps, and any
machine with that file gets the same software.

```bash
$ uptrack add anomalyco/opencode
tracking opencode  anomalyco/opencode
  latest v1.19.3, asset opencode-linux-x64.tar.gz
run uptrack install opencode to install it

$ uptrack install opencode
install opencode v1.19.3 (60.7 MB) to ~/.local/opt/opencode? [y/N] y
installed opencode v1.19.3 → ~/.local/opt/opencode
```

## Why

Some apps live outside every distro repo: upstream GitHub releases,
AppImages, standalone tarballs. Tracking them by hand means revisiting
release pages. uptrack does the boring part — watch the repo, pick the
right file for your system, verify it, and put it on your `PATH`.

## Features

- **Automatic asset selection** — OS, arch and libc filtering with
  format preference. It asks on ambiguity instead of guessing.
- **Verified downloads** — sha256 checked against the digest GitHub
  publishes; missing digests are labeled, never silently trusted.
- **Safe installs** — atomic swap keeps the old version live until the
  new one is staged; archives are extracted with traversal protection.
- **`.deb` without root** — extracts the payload into `~/.local`,
  never touches dpkg or maintainer scripts.
- **Dotfiles-friendly** — three required fields per app, machine state
  kept separate, `uptrack sync` restores everything on a new box.

## Install

Prebuilt Linux binaries (x86_64, aarch64) are on the
[releases page](https://github.com/vaproh/uptrack/releases), or build
from source (Go 1.27+):

```bash
go install github.com/vaproh/uptrack/cmd/uptrack@latest
```

`.tar.xz` and `.tar.zst` extraction shells out to the system `xz` and
`zstd` binaries. Everything else is stdlib; dependencies are `cobra`
and `yaml.v3` only.

## Usage

```bash
uptrack add <owner/repo|url> [--channel stable|prerelease|any]
uptrack list                       # tracked apps + versions
uptrack check                      # refresh release info
uptrack outdated                   # apps needing attention
uptrack info <name>                # release notes + why this asset
uptrack log <name>                 # release history
uptrack install <name> [--version <tag>]
uptrack sync                       # install missing + upgrade outdated
uptrack remove <name> [--purge]    # stop tracking, optionally delete files
```

Installs and upgrades confirm unless `-y`. Colors follow the terminal;
`NO_COLOR` and `--no-color` disable them.

## Config

`~/.config/uptrack/config.yaml`. Three fields required, rest inferred:

```yaml
apps:
  - name: opencode
    repo: anomalyco/opencode
    channel: stable
```

Full schema, channels and environment variables: [docs/config.md](docs/config.md).

## Docs

- [config](docs/config.md) — files, schema, channels, environment, dotfiles workflow
- [selection](docs/selection.md) — how the right asset is picked, with a worked example
- [install](docs/install.md) — pipeline, formats, `.deb` semantics, archive safety
- [PRD](PRD.md) — product decisions and non-goals
- [AGENTS](AGENTS.md) — conventions for automated contributors

## Limits

- GitHub releases only. No rpm support. AppImages need FUSE.
- `.deb` dependencies are not resolved — the post-install `ldd` check
  names missing libraries for your system package manager.
- Unauthenticated API: 60 req/hour; set `GITHUB_TOKEN` to raise it.

## Development

```bash
go build ./...   # build
go vet ./...     # vet
gofmt -l .       # must print nothing
go test ./...    # hermetic suite, no network
```

Cut a release with `git tag vX.Y.Z && git push origin vX.Y.Z` — CI
gates on checks, then builds both binaries, generates the changelog
from conventional commits, and publishes the GitHub release.

## License

MIT. See [LICENSE](LICENSE).
