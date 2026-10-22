# Asset selection

Upstream renames assets between releases, so selection re-runs on every
release instead of recording a filename. It is a pipeline of reject
filters followed by ranking. It never guesses: a tie asks on a terminal
and errors otherwise.

## Filters, in order

1. **Format.** Keep raw binaries, `tar.gz`/`tgz`, `tar.bz2`/`tbz2`,
   `tar.xz`/`txz`, `tar.zst`/`tzst`, `tar`, `zip`, `AppImage`, `deb`.
   Everything else goes: `yml`, `json`, checksums, signatures,
   blockmaps, `rpm`, `exe`, `dmg`, `apk`.
2. **OS.** Tokens mapping to another OS reject the asset: `darwin`,
   `macos`, `windows`, `win`, `freebsd`, etc. No OS token passes
   (universal asset).
3. **Arch.** Families: `amd64` (`x86_64`, `x64`), `arm64` (`aarch64`),
   `386` (`i686`), `arm` (`armv7l`). A token for another family
   rejects; no token passes. Multi-arch ("universal") binaries pass and
   would tie-break by format rank or ask.
4. **Libc.** `musl` assets are rejected on glibc hosts; `gnu`/`glibc`
   assets are rejected on musl hosts. No token passes.

## Ranking

Survivors are ranked by `(demotion, format)`:

- **Demotion 1:** `baseline` builds (run on older CPUs, still valid) and
  GUI builds (`desktop`, `gui` tokens). Demoted, never rejected.
- **Format:** raw binary (0) < archive (1) < AppImage (2) < deb (3).

The single best candidate wins. Multiple candidates tied at the top
are an ambiguity error listing the names.

## Overrides

```yaml
asset:
  match: "opencode-linux-x64*.tar.gz"  # glob, case-insensitive
  exclude: ["*musl*", "*baseline*"]
```

`match` filters before the pipeline; `exclude` drops after matching.
The opencode case is why both exist: v1.19.3 ships
`opencode-linux-x64.tar.gz` next to `-musl`, `-baseline` and
`-baseline-musl` variants, all matching the same glob.

A stored rule that matches nothing (upstream renamed the asset) falls
back to automatic selection with a warning, so configs self-heal. If
automatic selection ties after fallback, it asks or errors as usual.

## Worked example

Host: linux, x86_64, glibc. opencode v1.19.3, 24 assets:

| Asset | Verdict |
|---|---|
| `opencode-linux-x64.tar.gz` | **winner** |
| `opencode-linux-x64-baseline.tar.gz` | demoted, older CPUs |
| `opencode-desktop-linux-x86_64.AppImage` | demoted, GUI build |
| `opencode-desktop-linux-amd64.deb` | demoted, GUI build |
| `opencode-linux-x64-musl.tar.gz` | rejected, musl on glibc |
| `opencode-linux-arm64.tar.gz` | rejected, wrong arch |
| `opencode-darwin-x64.zip`, `...-windows-...` | rejected, wrong OS |
| `latest-linux.yml`, `latest.json`, `*.blockmap` | rejected, not installable |

See `uptrack info <name>`: it prints this reasoning per asset for the
latest release.
