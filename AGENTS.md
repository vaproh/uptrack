# AGENTS.md — working on uptrack

Personal CLI, one user. Keep it small: deletion beats addition.

## Commands

```bash
go build ./...        # build
go vet ./...          # vet
gofmt -l .            # must print nothing
go test ./...         # full suite, hermetic (httptest only, no network)
```

All four must pass before any commit.

## Layout

- `cmd/uptrack/main.go` — composition root, nothing else.
- `internal/config` — `config.yaml` (user YAML, dotfiles) + `state.json`
  (machine-local). Both written atomically (temp file + rename).
- `internal/gh` — GitHub releases API only. `GITHUB_TOKEN` honored,
  `UPTRACK_GITHUB_API` overrides the root.
- `internal/platform` — host os/arch/libc detection.
- `internal/asset` — selection engine: reject filters, then
  (demotion, format) ranking. Ties are errors, never guesses.
- `internal/version` — tolerant tag comparison.
- `internal/install` — download, verify, extract, place, symlink.
  No prompts, no output except `ldd` warnings.
- `internal/color` — ANSI helpers. Only package that knows about TTY.
- `internal/prompt` — stdin confirm/choose. TTY-only, fail otherwise.
- `internal/cli` — cobra commands. Only place that prints, prompts,
  or uses color.

## Rules

- Stdlib first. Current third-party deps: `cobra`, `yaml.v3`. New deps
  need a one-sentence justification in the commit message.
- Errors are values with context (`%w`). Sentinel errors where callers
  branch (`config.ErrNotFound`, `asset.AmbiguousError` via `errors.As`).
- Prompts and confirmations live in `cli` only. `install` stays
  non-interactive; `-y` is threaded through `ctx.assume`.
- Selection changes must keep the real-asset table test in
  `internal/asset/asset_test.go` passing (opencode v1.19.3 fixture).
- Extractor changes must keep the traversal-rejection tests passing.
  New archive support needs: format detection in `detect.go`, a stream
  in `extract.go`, exe location in `exe.go`, and a fixture test.
- New commands need a path through the CLI smoke test in
  `internal/cli/cli_test.go` where feasible.
- No network in tests. Ever.
- Product decisions live in `PRD.md`. If a change contradicts it, say
  so instead of silently drifting.

## Releases

Cut with `git tag vX.Y.Z && git push origin vX.Y.Z`. The release
workflow gates on gofmt/vet/test, builds linux x86_64 + aarch64,
generates the changelog from conventional commits via git-cliff
(`cliff.toml`), and publishes binaries + `CHANGELOG.md` +
`sha256sums.txt`. Write commit messages accordingly: `feat:`, `fix:`,
`docs:` group the notes; anything else lands under Miscellaneous.
