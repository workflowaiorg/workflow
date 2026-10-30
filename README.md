# wf

wf sets up a way of working with AI coding assistants (Claude Code, GitHub Copilot, opencode) in a
project, and keeps it the same for every assistant and every person on the team.

What a project gets from it:

- **Instructions the assistants read**: skills, slash commands and subagents. They are written once,
  and wf copies them into the folder each assistant reads.
- **A workflow around them**: every change starts from an Issue, the assistant works in a worktree of
  its own, and nothing is committed until a person has read the work and said go.
- **Checks** that the copies were not edited by hand and that the project's specs and decision
  records agree with each other.

> wf is young. It works with GitLab only so far, and it can't update a project to a newer wf yet.
> The milestone [v0.1.0](https://github.com/workflowaiorg/workflow/milestone/1) lists what comes
> next.

## Contents

- [Install](#install)
- [Start in a project](#start-in-a-project)
- [A day with wf](#a-day-with-wf)
- [Your rules and wf's rules](#your-rules-and-wfs-rules)
- [Settings: .wf.yml](#settings-wfyml)
- [Commands](#commands)
- [Where everything lives](#where-everything-lives)
- [When something goes wrong](#when-something-goes-wrong)

## Install

```sh
go install github.com/workflowaiorg/workflow/cmd/wf@latest
```

Or download a binary from [Releases](https://github.com/workflowaiorg/workflow/releases), and check
it against the release's `checksums.txt`.

Check that it works:

```sh
wf version
```

## Start in a project

Run this at the root of the repository:

```sh
wf init --forge=gitlab
```

The forge is where the project's Issues and merge requests live. If you leave out `--forge`, wf
asks for it.

`wf init` writes:

| What | Where |
|---|---|
| wf's settings for the project | `.wf.yml` |
| wf's part of every instruction | `.agents/base/` |
| every assistant's copy of them | `AGENTS.md`, `.claude/`, `.github/`, `.opencode/` |
| the working guide and two templates | `.workflow/GUIDE.md`, `.workflow/SPEC-TEMPLATE.md`, `.workflow/DECISION-TEMPLATE.md` |
| an empty index of decision records | `.workflow/decisions/README.md` (yours from then on) |
| what `wf sync` wrote, so it knows its own files | `.wf.lock` |

Then:

1. Read `.workflow/GUIDE.md`. It is the short manual for the workflow.
2. Open `.wf.yml`. It has commented examples. Uncomment what your project needs (see
   [Settings](#settings-wfyml)).
3. Commit everything `wf init` wrote. The assistants read these files from the repository.
4. Add `/.claude/worktrees` to `.gitignore`. That is where `wf worktree` puts worktrees.
5. Optional: run `wf sync --check` and `wf spec-coverage` in CI, so the checks bind everyone and
   not only the people who run them.

`wf init` refuses to run in a project that already has a `.wf.yml`.

### If the project already has instructions

Many projects already have an `AGENTS.md`, a `.claude/CLAUDE.md` or a
`.github/copilot-instructions.md`. `wf init` doesn't overwrite them. It lists each one as `skipped`,
writes everything else, and fails. Your text is still there. To finish:

1. Move what you want to keep into `.agents/project/AGENTS.md`. wf adds it to its own instructions
   under `## This project`. If the same text sits in more than one of these files, keep one copy.
2. Replace the old files with the generated ones:

   ```sh
   wf sync --force
   ```

   It prints each file it replaced.
3. Check the result in `AGENTS.md`, then commit.

Don't run `wf init` again: `.wf.yml` exists now, so it would refuse. `wf sync` does the rest.

Skills, commands and subagents of your own work the same way. Move each into `.agents/project/`
(see [Your rules and wf's rules](#your-rules-and-wfs-rules)) before `wf sync --force` replaces the
copy at the same path.

## A day with wf

You talk to the assistant, and the assistant follows the `workflow` skill. Here is what that looks
like.

1. **You ask for a change.** The assistant finds the Issue for it, or opens one before it edits
   anything.
2. **The assistant gets a worktree.** It runs `wf worktree 42 add-login` and works in the folder it
   prints, on the branch `42-add-login`. Your main checkout stays on `main`, so several Issues can
   be worked on at once.
3. **The assistant works and tells you what it did.** Nothing is committed yet.
4. **You run `/pre-mr-check`** (or the assistant offers it). It runs the checks the changed files
   call for. Then a reviewer subagent reads the change with fresh eyes. You get a table and a verdict.
5. **You say go.** Only then does the assistant commit, push and open the merge request.
6. **Merge.** The Issue closes.

Bigger work starts with a plan. The assistant writes it in `wf worktree plan <slug>`. Then
`/plan-check` has a reviewer read it, and you accept it before any of its Issues start.

The full rules are in the `workflow` skill. The short version is in `.workflow/GUIDE.md`.

## Your rules and wf's rules

Every instruction has two parts:

- **wf's part** lives in `.agents/base/`. wf writes it. **Never edit it.** `wf sync --check` fails
  if you do.
- **Your part** lives in `.agents/project/`, in the same layout. wf never touches it. Where the two
  disagree, yours wins.

`wf sync` joins the two parts and writes the result into every assistant's folder. Your part goes
at the end, under a heading `## This project`.

**Example.** To tell the `workflow` skill that your project's areas are `api` and `web`, create
`.agents/project/skills/workflow/SKILL.md`:

```markdown
Area labels: `area::api`, `area::web`.
```

Then run:

```sh
wf sync
```

Now `.claude/skills/workflow/SKILL.md`, `.github/skills/workflow/SKILL.md` and
`.opencode/skills/workflow/SKILL.md` all end with your line. To see the result without opening a
file:

```sh
wf show workflow          # the skill as an assistant reads it
wf show workflow --base   # wf's part alone
```

**A skill, command or subagent of your own** is a file in `.agents/project/` with no base part,
such as `.agents/project/skills/deploy/SKILL.md`. It needs a `description` in its frontmatter, the
line an assistant reads to decide when to use it. `wf sync` copies it to every assistant like the
others.

**Frontmatter.** A part may start with YAML frontmatter between `---` lines. Your keys replace wf's
keys, one by one. For example, you can change a skill's `description` and keep everything else. A
subagent's frontmatter holds only its `description`. wf writes each assistant's permissions itself,
and every subagent only reads and reports.

**The two templates take no part of yours.** `.workflow/SPEC-TEMPLATE.md` and
`.workflow/DECISION-TEMPLATE.md` are copied whole into every new spec and record, so anything added
to them would land in each one. `wf sync` refuses `.agents/project/workflow/SPEC-TEMPLATE.md` and
`.agents/project/workflow/DECISION-TEMPLATE.md`. Say what your project's specs need, such as which contracts
they point to, in `.agents/project/skills/domain-specs/SKILL.md`.

**Never edit a generated file.** Every file wf writes starts with a line that says so. The next
`wf sync` overwrites your edit, and `wf sync --check` fails on it. Put the change in
`.agents/project/` instead.

## Settings: .wf.yml

```yaml
# wf's version this project uses, and the forge its Issues live on. wf init writes both.
wf: 0.1.0
forge: gitlab

# Files git ignores that a new worktree needs, such as env files. Globs, from the project root.
worktree:
  copy: [".env*", "web/.env*"]
  skip: ["*.example"]          # matched against the path and against the file name

# The project's own checks for /pre-mr-check. Each runs when a changed file matches its paths.
gate:
  - name: lint
    paths: ["**/*.go"]
    run: golangci-lint run ./...
  - name: tests
    paths: ["**/*.go", "go.mod"]
    run: go test ./...
```

- `wf` and `forge` are required. `wf sync`, `wf show` and `wf worktree` check every setting first,
  name every mistake at once, and change nothing when there is one.
- `worktree:` is optional. wf copies only files git ignores. A file git tracks is already in the
  worktree, and copying an untracked one would make the worktree look changed.
- `gate:` is optional. wf's two checks, `wf sync --check` and `wf spec-coverage`, are always in the
  gate. After you edit `gate:`, run `wf sync`: the gate table is part of the `/pre-mr-check` command
  that wf writes.

## Commands

Run `wf help` for the list, or `wf <command> -h` for one command.

### `wf init [--forge <forge>]`

Starts using wf in a project. See [Start in a project](#start-in-a-project).

### `wf sync`

Writes every assistant's files from `.agents/base/` and `.agents/project/`. Run it after you change
anything in `.agents/project/` or `.wf.yml`. Commit what it writes.

It prints each file it wrote or removed. It also records what it wrote in `.wf.lock`. That is how
it knows which files are its own. Don't edit `.wf.lock`.

If a file wf wants to write is already there and wf didn't write it, wf leaves it alone, lists it
and fails. `wf sync --force` replaces such files.

### `wf sync --check`

Writes nothing. It fails, and names each file, when:

- a generated file is not what `wf sync` would write: someone edited it by hand, or forgot to run
  `wf sync`;
- `.agents/base/` is not wf's part for this version: someone edited it, or a file is missing or
  extra;
- `.wf.yml` pins a different wf version from the one running. Run the version it pins.

This is the check to run in CI.

### `wf show <skill> [--base]`

Prints a skill as the assistants read it, both parts joined. `--base` prints wf's part alone.

### `wf worktree <issue> <slug>`

Gives an Issue its own worktree, or finds the one it already has.

```sh
wf worktree 42 add-login
```

- If the Issue has no branch yet, wf creates `.claude/worktrees/42-add-login` on a new branch
  `42-add-login` from `origin/main`.
- If a branch `42-…` already exists, wf uses it, whatever its slug.
- If that branch is already checked out somewhere, wf prints where.
- If it finds two branches for one Issue, it stops and lists them. You choose.

wf copies the files `.wf.yml` lists under `worktree:` into the new worktree. Notes go to stderr. The
worktree's path is printed last, on its own line.

### `wf worktree plan <slug>`

The same, for a plan. The branch is exactly `plan-<slug>`.

### `wf spec-coverage [--report-only]`

Checks that the project's specs and decision records agree:

- every `[ADR-NNNN]` link in a spec points at a record that is still in force;
- every accepted record is cited by its spec and listed in `.workflow/decisions/README.md`, and its
  index row says what the record says;
- every plan folder is named `<DD-MM-YYYY>-<slug>`.

It also counts plans that have been open more than 30 days as debt. Debt is reported, never failed
on.

`--report-only` prints the same report and exits 0. Use it while you are cleaning up.

### `wf version`, `wf help [--json]`

`wf version` prints the version. `wf help` lists the commands. `--json` gives the list in a form an
agent can read.

### Exit codes

| Code | Means |
|---|---|
| 0 | done |
| 1 | a check failed, or something went wrong |
| 2 | the command was used wrongly: a typo, a missing flag, a missing setting in `.wf.yml` |

wf asks a question only when it runs in a terminal. In a script or CI it fails instead, and names
the flag to pass.

## Where everything lives

```
.wf.yml                              your settings
.wf.lock                             what wf sync last wrote; never edited
.agents/base/                        wf's part; never edited
.agents/project/                     your part, in the same layout:
  AGENTS.md                            → AGENTS.md, .claude/CLAUDE.md, .github/copilot-instructions.md
  workflow/GUIDE.md                    → .workflow/GUIDE.md
  skills/<name>/SKILL.md               → .claude/skills/, .github/skills/, .opencode/skills/
  commands/<name>.md                   → .claude/commands/, .github/prompts/, .opencode/commands/
  subagents/<name>.md                  → .claude/agents/, .github/agents/, .opencode/agents/
.workflow/
  GUIDE.md                           the working guide
  SPEC-TEMPLATE.md                   copy it to start a domain's spec
  DECISION-TEMPLATE.md               copy it to start a decision record
  specs/<domain>/SPEC.md             what is true of a domain now
  decisions/NNNN-<slug>.md           why, and what was turned down
  decisions/README.md                the index of decision records
  plans/<DD-MM-YYYY>-<slug>/PLAN.md  a plan in progress
  plans/archive/                     finished plans
.claude/worktrees/                   worktrees made by wf worktree; gitignore it
```

What wf installs:

| Kind | Name | What it is for |
|---|---|---|
| skill | `workflow` | how every change goes: Issue, worktree, check, go, merge |
| skill | `forge-gitlab` | the GitLab commands and labels the `workflow` skill uses |
| skill | `domain-specs` | how to write specs and decision records |
| command | `/plan-check` | a reviewer subagent reads a plan before any work starts |
| command | `/pre-mr-check` | runs the gate, then a reviewer subagent reads the change |
| subagent | `plan-reviewer` | reviews plans; reports, never edits |
| subagent | `code-reviewer` | reviews changes; reports, never edits |

## When something goes wrong

| wf says | Do this |
|---|---|
| `<file>: differs from what wf sync writes` | Move your edit to `.agents/project/`, then run `wf sync` |
| `<file>: missing`, `no longer generated`, `.wf.lock: out of date` | Run `wf sync` and commit what it writes |
| `.agents/base/…: differs from wf vX's base` | Restore the file (`git checkout -- .agents/base`). Put your change in `.agents/project/` |
| `pins wf vX, but this is wf vY` | Install the version `.wf.yml` pins |
| `skipped … (not written by wf)` | A file of your own is at a path wf writes to. Move it, or run `wf sync --force` to replace it |
| `kept … (no longer generated, but edited by hand)` | wf no longer writes this file, and someone edited it. Delete it yourself if you don't need it |
| `a template takes no project part` | Move that text into `.agents/project/skills/domain-specs/SKILL.md` and delete the file |
| `.wf.yml: pins no wf version` | Add `wf:` with the version `wf version` prints, such as `wf: 0.1.0` |
| `.wf.yml: names no forge`, `forge: … is not one wf has a base for` | Set `forge:` to a forge wf knows: `gitlab` |
| `.wf.yml: gate[0] (lint) needs …` | Give that step what it names: `name:`, `paths:` as a list, `run:` |
| `.wf.yml: worktree.copy …`, `worktree.skip …` | Make it a list of globs, such as `[".env*"]`, and check each pattern |
| `… uses <key>, which .wf.yml does not set` | Add the key it names to `.wf.yml` |
| `has more than one branch` (worktree) | Delete the branch you don't want, or work in the one you do |

## Licence

[MIT](LICENSE)
