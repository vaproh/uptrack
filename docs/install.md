# Install pipeline

`install <name>` and `sync` share one pipeline. Every step is ordered
so a failure never leaves a broken install behind.

## Steps

1. **Resolve** the release: latest for the channel, or `--version <tag>`.
2. **Select** the asset (see [selection](selection.md)).
3. **Download** to a temp file, streaming. The byte count must match the
   size in the release metadata when published.
4. **Verify** sha256. The asset's own `digest` field wins. Without
   one, uptrack looks for `<asset>.sha256` then `SHA256SUMS`-style
   files on the same release, in both `<hex>  <file>` and
   `SHA256 (file) = <hex>` forms. A match verifies the same way;
   nothing found means the install is labeled **unverified**.
   A mismatch aborts and deletes the temp file.
5. **Extract** into a fresh staging dir (archives and debs only).
6. **Swap**: stage to `<dest>.new`, rename current `<dest>` aside,
   rename staged into place, remove the old tree. The previous install
   stays live until the swap succeeds.
7. **chmod +x** the executable, **symlink** it to `~/.local/bin/<name>`.
8. **Record** installed version, asset name and verified flag in state.
9. **ldd check**: ELF binaries with unresolvable shared libraries
   produce a warning naming them.

Rollback is `install --version <older-tag>`. There is no rollback
command by design.

## Formats

| Input | Handling |
|---|---|
| Raw binary | chmod +x, move into place. |
| AppImage | chmod +x, move into place. Needs FUSE on the host. |
| `tar.gz`, `tar.bz2`, `zip`, `tar` | Go stdlib extraction. |
| `tar.xz`, `tar.zst` | System `xz -dc` / `zstd -dc` piped into the tar reader. Clear error naming the missing binary. |
| `deb` | `data.tar.*` extracted into the prefix (see below). |

Unsupported: `rpm`, self-extracting `.run`, anything needing root.

## Finding the executable

- Explicit `install.exe` always wins (path relative to the install dir
  for archives/debs).
- Archives: a single file at the root, else descend one top-level dir,
  else exactly one executable file — otherwise the install errors and
  names `install.exe` as the fix.
- Debs: the `.desktop` file's `Exec` line first, then a lone binary in
  `usr/bin` / `bin`.

## .deb semantics

A `.deb` here is an `ar` container holding a file tree. uptrack parses
the `ar` headers itself (~50 lines, no dependency), extracts
`data.tar.*` into `~/.local/opt/<name>/`, and ignores everything else:

- Maintainer scripts never run.
- Nothing touches the dpkg database or requires root.
- Dependencies are **not** resolved. An app needing `libgtk-4` extracts
  fine and fails at launch; the post-install `ldd` warning names the
  missing library so you can install it with your system package
  manager.

## Desktop integration

`.deb` and AppImage installs copy `.desktop` launchers into
`~/.local/share/applications` (honors `XDG_DATA_HOME`), rewrite `Exec`
to the installed binary, and resolve packaged icons. AppImage
extraction needs FUSE; when it fails the install still succeeds with
a note. `uninstall` removes the launcher files. `--no-desktop` opts
out per install. `update-desktop-database` runs when present.

## Archive safety

- Absolute paths and `..` escapes abort the install.
- Symlinks resolving outside the install dir are dropped.
- Devices, fifos and hardlinks are skipped.
- Permissions come from the archive, masked to `0777`; the detected
  executable is forced `0755`.
