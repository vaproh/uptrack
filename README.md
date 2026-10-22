# uptrack

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
  publishes, else `<asset>.sha256` / `SHA256SUMS` files on the same
  release. Missing both means labeled unverified, never silently
  trusted.
- **Safe installs** — atomic swap keeps the old version live until the
  new one is staged; archives are extracted with traversal protection.
- **`.deb` without root** — extracts the payload into `~/.local`,
  never touches dpkg or maintainer scripts.
- **Desktop launchers** — `.deb` and AppImage installs appear in your
  launcher; `uninstall` removes them, `--no-desktop` skips them.
- **Pin versions** — hold an app back from `sync` until you are ready.
- **Dotfiles-friendly** — three required fields per app, machine state
  kept separate, `uptrack sync` restores everything on a new box.

## Install

One line, no root needed:

```bash
curl -fsSL https://raw.githubusercontent.com/vaproh/uptrack/main/install.sh | sh
```

Detects your architecture, downloads the latest release with checksum
verification, and places `uptrack` in `~/.local/bin`.
`INSTALL_DIR` overrides the destination, `UPTRACK_VERSION=v0.1.0` pins
a version.

Or build from source (Go 1.27+):

```bash
go install github.com/vaproh/uptrack/cmd/uptrack@latest
```

`.tar.xz` and `.tar.zst` extraction shells out to the system `xz` and
`zstd` binaries. Everything else is stdlib; dependencies are `cobra`
and `yaml.v3` only.

## Updating

```bash
uptrack self-update        # replace this binary with the latest release
uptrack self-update -y     # no confirmation
uptrack version            # what you are running
```

`self-update` verifies the download against the release's
`sha256sums.txt` before replacing the binary. The curl installer above
is only needed once.

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
uptrack self-update                # update uptrack itself
uptrack version                    # print version
uptrack remove <name>               # stop tracking, keep files
uptrack uninstall <name> [--keep-files]   # asks per component what to remove
uptrack uninstall-self [--all]      # remove uptrack itself
uptrack pin <name> [tag]            # hold a version, sync skips it
uptrack unpin <name>                # follow the channel again
uptrack doctor                      # verify installs are healthy
uptrack completion [bash|zsh|fish]  # shell completions
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
