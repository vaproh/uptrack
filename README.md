# uptrack

![ci](https://github.com/vaproh/uptrack/actions/workflows/ci/badge.svg)

Track and install standalone Linux apps from GitHub releases. You add a
repo, uptrack picks the right file for your system, verifies it, and
installs it under `~/.local`. Copy `config.yaml` to a new machine, run
`uptrack sync`, and everything is there.

## Install

Prebuilt Linux binaries (x86_64, aarch64) are on the
[releases page](https://github.com/vaproh/uptrack/releases), or build
from source:

```bash
go install github.com/vaproh/uptrack/cmd/uptrack@latest
# or
git clone https://github.com/vaproh/uptrack && cd uptrack
go build -o ~/.local/bin/uptrack ./cmd/uptrack
```

Needs Go 1.27+. Two dependencies: `cobra`, `gopkg.in/yaml.v3`.
Extracting `.tar.xz` and `.tar.zst` shells out to the system `xz` and
`zstd` binaries.

## Usage

```bash
uptrack add anomalyco/opencode        # track a repo
uptrack add https://github.com/owner/repo --channel prerelease
uptrack list                            # tracked apps
uptrack check                           # refresh release info
uptrack outdated                        # only apps needing attention
uptrack info opencode                   # release notes + asset reasoning
uptrack log opencode                    # release history
uptrack install opencode                # install or upgrade
uptrack install opencode --version v1.19.2
uptrack sync                            # install missing + upgrade outdated
uptrack remove opencode                 # stop tracking (files stay)
uptrack remove opencode --purge         # also delete files
```

Installs and upgrades ask for confirmation; `-y` / `--yes` skips it.
`--no-color` disables styling; `NO_COLOR` is honored too.

## Config

`~/.config/uptrack/config.yaml` (XDG-aware). Only three fields are
required; the rest is inferred:

```yaml
apps:
  - name: opencode          # required
    repo: anomalyco/opencode # required
    channel: stable          # required: stable | prerelease | any
    asset:                   # optional override
      match: "opencode-linux-x64*.tar.gz"
      exclude: ["*musl*", "*baseline*"]
    install:                 # optional override
      type: archive          # binary | archive | appimage | deb
      dest: ~/.local/opt/opencode
      exe: opencode
```

Machine-local state (installed versions, latest seen) lives in
`~/.local/state/uptrack/state.json`, so the YAML stays clean for
dotfiles. Apps install to `~/.local/opt/<name>/` with a symlink at
`~/.local/bin/<name>`.

## How asset selection works

Selection re-runs on every release, since upstream renames assets
between versions:

1. Keep installable formats (raw binary, tar.gz/bz2/xz/zst, zip,
   AppImage, deb). Metadata files (`.yml`, `.json`, checksums,
   blockmaps) are dropped first.
2. Reject other OSes (darwin, windows, freebsd) and other arches.
3. Reject musl builds on glibc hosts and glibc builds on musl hosts.
4. Deprioritize `baseline` builds and GUI builds (`desktop`, `gui`).
5. Prefer raw binary over archive over AppImage over deb.

One winner installs silently. A tie asks on a terminal and errors
otherwise; it never guesses. `asset.match` / `asset.exclude` override
the automatic pick, and a stored rule that matches nothing falls back
to automatic selection with a warning.

## Safety

- Downloads go to a temp file with a size check against the release
  metadata, then sha256 is verified against the digest GitHub
  publishes. No digest means the install is labeled **unverified**,
  never silently trusted. A mismatch aborts and deletes the file.
- Extraction rejects absolute paths and `..` escapes; symlinks
  pointing outside the install dir are dropped.
- The old install stays in place until the new tree is staged, then a
  rename swap replaces it.
- `.deb` files are extracted, never installed as root: no maintainer
  scripts, no dpkg database, no dependency resolution. `ldd` warns
  about missing shared libraries after install.
- No rollback command: `install --version <tag>` reinstalls any older
  release, which covers the same need with no extra machinery.

## Limits

- GitHub only (GitLab and generic URLs are future work).
- `.rpm` is not supported.
- AppImages need FUSE on the host.
- Asset naming is a convention, not a standard. When auto-selection
  gets it wrong, pin it with `asset.match`.
- Unauthenticated GitHub API is 60 requests/hour. Set `GITHUB_TOKEN`
  to raise it. `UPTRACK_GITHUB_API` overrides the API root (GitHub
  Enterprise).
