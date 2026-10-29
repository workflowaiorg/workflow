# wf

An AI-assisted engineering workflow for Go projects, as one CLI.

wf gives a project the skills, slash commands and subagents that AI coding assistants (Claude
Code, GitHub Copilot, opencode) read; a planning and review workflow on GitLab or GitHub, where every change
starts from an Issue and nothing is committed before the developer says go; and optional developer
commands for a Go backend on docker compose. A project adds only the parts it needs and updates
them later without losing its own rules.

> wf is at its very beginning: only `wf sync`, `wf show`, `wf version` and `wf help` exist. The milestone
> [v0.1.0](https://github.com/workflowaiorg/workflow/milestone/1) lists what comes first.

## Install

```sh
go install github.com/workflowaiorg/workflow/cmd/wf@latest
```

or download a binary from [Releases](https://github.com/workflowaiorg/workflow/releases) and check
it against the release's `checksums.txt`.

## Layout

```
.wf.yml                         the project's choices: the data that base files are rendered with
.wf.lock                        what wf sync last wrote, with a hash of each file; never edited
.agents/base/                   wf's part, never edited
.agents/project/                your part, in the same layout
  AGENTS.md                     → AGENTS.md, .claude/CLAUDE.md, .github/copilot-instructions.md
  GUIDE.md                      → .workflow/GUIDE.md
  skills/<name>/SKILL.md        → .claude/skills, .github/skills, .opencode/skills
  commands/<name>.md            → .claude/commands, .github/prompts, .opencode/commands
  subagents/<name>.md           → .claude/agents, .github/agents, .opencode/agents
```

`wf sync` writes every file on the right from both parts; `wf sync --check` fails, naming the
file, when one differs. A file already at one of those paths that wf did not write is left alone
and listed, and `wf sync --force` replaces it.

A part may start with YAML frontmatter, and the project's keys replace the base's. A subagent's
frontmatter holds only its `description`: wf writes each assistant's permissions, and every
subagent reports and never edits.

Base files are Go templates with `[[ ]]` as delimiters and `.wf.yml` as data, so `[[ .name ]]`
is the project's name; a literal `[[` is written `[[ "[[" ]]`. Project files are plain Markdown.

## Two rules

- **Generated files are never edited by hand.** wf writes every assistant's copy of a skill from
  one source; an edit to a copy is overwritten and fails the check in CI.
- **A project's own rules win.** Every skill has a base that wf owns and updates, and a project
  part that is yours and that wf never touches. Where they disagree, the project part wins.

## Licence

[MIT](LICENSE)
