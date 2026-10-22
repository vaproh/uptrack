# Configuration

uptrack reads one YAML file and keeps machine-local state elsewhere, so
the YAML stays clean for dotfiles.

## Files

| File | Purpose |
|---|---|
| `~/.config/uptrack/config.yaml` | Tracked apps. Commit this. Honors `XDG_CONFIG_HOME`. |
| `~/.local/state/uptrack/state.json` | Installed/latest versions. Never commit. |
| `~/.local/opt/<name>/` | App files. |
| `~/.local/bin/<name>` | Symlink to the app executable. |

If `~/.local/bin` is not on your `PATH`, uptrack prints the exact
`export` line to add after an install.

## Schema

```yaml
apps:
  - name: opencode          # required, local identifier
    repo: anomalyco/opencode # required, owner/repo
    channel: stable          # required: stable | prerelease | any
    pin: v1.19.2             # optional: hold this version, sync skips it
    asset:                   # optional, overrides auto-selection
      match: "opencode-linux-x64*.tar.gz"  # glob, case-insensitive
      exclude: ["*musl*", "*baseline*"]    # globs, optional
    install:                 # optional, overrides install inference
      type: archive          # binary | archive | appimage | deb
      dest: ~/.local/opt/opencode  # default shown
      exe: opencode          # binary name or path inside the archive
```

Only `name`, `repo` and `channel` are required. Everything else is
inferred at install time and only needs setting when inference gets it
wrong.

## Channels

- `stable` — newest non-prerelease. What you want 99% of the time.
- `prerelease` — newest release including prereleases. Follows beta/RC
  tags; expect breakage.
- `any` — same as `prerelease` today. Kept for future channel kinds
  (nightly builds etc.) without a schema change.

## Environment

| Variable | Effect |
|---|---|
| `GITHUB_TOKEN` | Authenticated API calls. Raises the rate limit from 60 req/hour. Never logged. |
| `UPTRACK_GITHUB_API` | Override the API root. Used for tests and GitHub Enterprise Server. |
| `NO_COLOR` | Disable colored output (also `--no-color`). |
| `XDG_CONFIG_HOME` | Moves `config.yaml` (stdlib behavior via `os.UserConfigDir`). |

## Workflows

New machine from dotfiles:

```bash
uptrack sync -y
```

Everything in the config gets installed or upgraded to the latest
release for its channel. `sync` without `-y` prints a plan
(install/upgrade per app) and asks once.
