# wf

An AI-assisted engineering workflow for Go projects, as one CLI.

wf gives a project the skills, slash commands and subagents that AI coding assistants (Claude
Code, GitHub Copilot, opencode) read; a planning and review workflow on GitLab or GitHub, where every change
starts from an Issue and nothing is committed before the developer says go; and optional developer
commands for a Go backend on docker compose. A project adds only the parts it needs and updates
them later without losing its own rules.

> wf is at its very beginning: only `wf version` and `wf help` exist. The milestone
> [v0.1.0](https://github.com/workflowaiorg/workflow/milestone/1) lists what comes first.

## Install

```sh
go install github.com/workflowaiorg/workflow/cmd/wf@latest
```

or download a binary from [Releases](https://github.com/workflowaiorg/workflow/releases) and check
it against the release's `checksums.txt`.

## Two rules

- **Generated files are never edited by hand.** wf writes every assistant's copy of a skill from
  one source; an edit to a copy is overwritten and fails the check in CI.
- **A project's own rules win.** Every skill has a base that wf owns and updates, and a project
  part that is yours and that wf never touches. Where they disagree, the project part wins.

## Licence

[MIT](LICENSE)
