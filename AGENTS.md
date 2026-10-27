# wf — instructions for AI assistants and contributors

wf is a Go CLI that brings an AI-assisted engineering workflow into Go projects: the skills,
commands and subagents that AI coding assistants read, the planning and review workflow around
them, and optional developer commands for a Go backend on docker compose. Its Issues live in this repo's
GitHub Issues; the milestone says which release each belongs to.

## How work is done

1. **An Issue before the first edit.** Read its description and comments first. A request with no
   Issue gets one before anything is edited, opening with `## In plain words`: what is missing,
   why it matters and what changes, in a few sentences a 10-year-old could follow.
2. **One Issue, one outcome, one PR.** Branch `<issue>-<short-slug>` from `main`; the PR says
   `Closes #n`. Work that turns up something else to fix opens another Issue instead of growing
   this one.
3. **Nothing is committed or pushed before the developer has read the work and said go.** Not a
   branch, not a draft PR. An approval covers the work it was given for; the next step needs its own.
4. **Before asking for go**, say what changed, what was checked (with the commands), what was not,
   and what you are unsure about.
5. **A question or concern about the design is answered in chat** with a recommendation and its
   trade-off. Nothing is rewritten until the developer agrees.
6. **Merging** is the developer's call. A PR with more than one commit is squashed only if they say so.

Commit messages: a summary line in plain words under ~72 characters, a blank line, then a list
of what was done. No AI signatures, no tool output.

## Code

- Go, at the version in `go.mod`; use the modern idioms it allows (`slices`, `maps`, `min`/`max`,
  range over integers, `errors.Is`/`As`).
- wf is a CLI: commands live in `internal/cli`, `cmd/wf` only starts it. A command writes to the
  writers it is given, never to `os.Stdout` directly, so it can be tested.
- Errors are wrapped with `%w` and say what was being done (`reading .wf.yml: …`). A user's mistake
  exits 2, a failure exits 1.
- Tests are in `package foo_test`, table-driven where there are cases. A helper only the tests need
  is exposed through `export_test.go`.
- `gofmt`, `go vet ./...` and `go test ./...` pass before a PR leaves draft; CI runs the same.
- No dependency is added without saying why in the PR.

## What wf must never contain

- **The name, paths or services of any project that uses it.** wf is generic: a base skill,
  template, test or doc names no particular product. Project-specific values live in that
  project's `.wf.yml` or its own skill parts. Tests use sample projects under `testdata/`, never a real project's files.
- Secrets or tokens of any kind.

## Releases

A tag `vX.Y.Z` on `main` runs `.github/workflows/release.yml`: GoReleaser builds the binaries and
attaches `checksums.txt`. Tags are pushed by the developer, or on their go.
