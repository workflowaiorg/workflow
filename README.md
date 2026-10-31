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
- **One way to run the project locally**: `wf up` and `wf down`, plus `wf doctor` and `wf scaffold`,
  which check a machine and take a clean clone to a running stack.

> wf is young. It works with GitLab only so far, and it can't update a project to a newer wf yet.
> The milestone [v0.1.0](https://github.com/workflowaiorg/workflow/milestone/1) lists what comes
> next.

## Contents

- [Install](#install)
- [Start in a project](#start-in-a-project)
- [A day with wf](#a-day-with-wf)
- [Your rules and wf's rules](#your-rules-and-wfs-rules)
- [Run the stack](#run-the-stack)
- [Components](#components)
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

### Every project runs the wf it pins

You install wf once, any version. In a project, it runs the version the project's `.wf.yml` pins
(`wf: 0.2.0`), not itself. So everyone on the team, and CI, generates the same files.

When you run a command, wf:

1. reads `wf:` in `.wf.yml`. No `.wf.yml`, no `wf:`, or the same version as itself: it just runs;
2. looks for that version in its cache, `wf/` in your user cache folder (`~/Library/Caches` on
   macOS; `$XDG_CACHE_HOME`, or else `~/.cache`, on Linux). If it is there, it runs it. No internet needed;
3. otherwise downloads it from [Releases](https://github.com/workflowaiorg/workflow/releases) and
   compares the download's fingerprint (its SHA-256) with the one in the project's `.wf.sum`. If
   they match, it keeps the copy and runs it. If not, it deletes the download and runs nothing.

`.wf.sum` is written by [`wf lock`](#wf-lock), once per version, and committed. A download is only
trusted if it matches what your team reviewed.

To run the wf you installed whatever the project pins, for example to try a build of wf itself:

```sh
WF_NO_LAUNCH=1 wf sync
```

Then the files it writes may differ from everyone else's.

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
3. Run `wf lock`. It records in `.wf.sum` what the wf version `.wf.yml` pins must look like when
   someone downloads it. `wf init` doesn't do this itself, because it needs the internet.
4. Commit everything `wf init` and `wf lock` wrote. The assistants read these files from the repository.
5. Add `/.claude/worktrees` to `.gitignore`. That is where `wf worktree` puts worktrees.
6. Optional: run `wf sync --check` and `wf spec-coverage` in CI, so the checks bind everyone and
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

## Run the stack

wf runs the project's stack with docker compose, on the compose file at the project's root and every
file it includes.

```sh
wf scaffold        # a clean clone to a running stack, in one command
wf up              # start everything, in the background
wf up api          # start api, and follow its logs; Ctrl-C stops it
wf down            # stop everything
wf doctor          # is this machine ready, and is the stack? Changes nothing
```

`wf doctor` runs checks, each a command that passes when it exits 0. They come from the components
you added and from `doctor:` in `.wf.yml` (see [Settings](#settings-wfyml)). Each gets a line:

```
✓ docker running
→ env file: wf scaffold fixes this with cp .env.example .env
✗ go installed
    sh: go: command not found
– api answers: skipped, fix the problems above first
```

- `✓` passed.
- `→` failed, but the check has a `fix:`, so `wf scaffold` fixes it. It does not fail `wf doctor`.
  What the check printed is under it, as for `✗`.
- `✗` failed, and you fix it by hand. What the check printed is under it. `wf doctor` exits 1.
- `–` was skipped. A check marked `when: up` runs only while the stack is up and every check above it
  passed. Otherwise its failure would only repeat theirs.

`wf scaffold` does what a newcomer would do by hand, in order:

1. runs the checks that do not need the stack. If one is `✗`, it stops and changes nothing;
2. runs the `fix:` of each `→`, then that check again, which must now pass;
3. starts the stack, as `wf up` does;
4. runs the `when: up` checks, fixing those that fail and have a `fix:`, such as migrations.

Run it again at any time: what already passes is left alone.

A `when: up` check runs as soon as the stack has started, before its services are ready, so a
check that waits for a service should retry on its own, as the `curl --retry` example below does.

## Components

A component adds one capability to the project, such as a backend or a frontend. It brings its own
skills, commands and sections of `AGENTS.md` and the guide; its own steps in `/pre-mr-check`'s gate;
its own checks in `wf doctor`; and its own wf commands, which start with its name: `wf <component> <command>`.

```sh
wf add                    # list the components wf carries
wf add <component>        # add one; wf asks what it needs to know
```

Every question is also a flag, so a script or an agent passes the answer instead of being asked.
`wf add` then:

1. puts the component's part of the base into `.agents/base`;
2. writes the files it gives a project once (yours from then on, like the decisions index);
3. records the component and your answers under `components:` in `.wf.yml`. Your settings and
   comments stay; blank lines between keys and your own indentation are tidied away;
4. runs `wf sync`.

When the component needs a change in a file your project owns, such as `.gitignore`, wf prints it on
a `by hand` line and leaves the change to you. Commit what `wf add` wrote, as after `wf init`.

wf carries no components yet; the backend and frontend come next. `wf add` lists what your version
has.

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

# The project's own checks for wf doctor and wf scaffold. A check passes when run: exits 0.
doctor:
  - name: env file
    run: test -f .env
    fix: cp .env.example .env    # optional: what wf scaffold runs when the check fails
  - name: api answers
    run: curl -sf --retry 10 --retry-connrefused http://localhost:8000/health
    when: up                     # optional: run only while the stack is up

# The components wf add installed, with the answers given. wf add writes this.
components:
  backend:
    grpc: true
```

- `wf` and `forge` are required. `wf sync`, `wf show` and `wf worktree` check every setting first,
  name every mistake at once, and change nothing when there is one.
- `worktree:` is optional. wf copies only files git ignores. A file git tracks is already in the
  worktree, and copying an untracked one would make the worktree look changed.
- `gate:` is optional. wf's two checks, `wf sync --check` and `wf spec-coverage`, are always in the
  gate. After you edit `gate:`, run `wf sync`: the gate table is part of the `/pre-mr-check` command
  that wf writes. A component's own steps come before yours.
- `doctor:` is optional. A component's checks come before yours, and checks marked `when: up` run after all the others. A command YAML would read as
  something else, such as `false`, goes in quotes: `run: "false"`.
- `components:` is written by `wf add`. To change an answer, edit it here and run `wf sync`.

## Commands

Run `wf help` for the list, or `wf <command> -h` for one command.

### `wf init [--forge <forge>]`

Starts using wf in a project. See [Start in a project](#start-in-a-project).

### `wf add [<component>] [--<option> …] [--force]`

Adds a component; with no component, lists them. `wf add <component> -h` lists its options and
the flag for each. See [Components](#components). `--force` is
passed on to its `wf sync`.

### `wf <component> <command>`

Runs a command of an installed component. `wf <component> -h` lists them, and so does `wf help`.

### `wf lock`

Records in `.wf.sum` the fingerprint (SHA-256) of the release `.wf.yml` pins, one line for each
platform wf runs on: macOS and Linux, Intel and ARM. It reads them from the release's
`checksums.txt`. Commit `.wf.sum`: every later download of that version is checked against it.

```
wf_0.2.0_darwin_amd64.tar.gz sha256:…
wf_0.2.0_darwin_arm64.tar.gz sha256:…
wf_0.2.0_linux_amd64.tar.gz sha256:…
wf_0.2.0_linux_arm64.tar.gz sha256:…
```

Run it after `wf init`, and again each time you change `wf:` in `.wf.yml`. It keeps only the
pinned version's lines.

- If `.wf.sum` already holds them, it does nothing and needs no internet.
- It never replaces a fingerprint already recorded with a different one. If the release no longer
  matches what was recorded, it fails and leaves `.wf.sum` alone.

`wf lock` always runs as the wf you installed, never as the pinned version: it writes the file the
pinned version is checked against. Don't edit `.wf.sum` by hand.

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

### `wf up [<service> …]`, `wf down`

`wf up` runs `docker compose up -d`: the whole stack, in the background. With services named, it
starts only those and stays attached to their logs. The names are the ones in the compose file.
Ctrl-C stops them, and wf waits until they have stopped. `wf down` runs `docker compose down`.

### `wf doctor`

Runs every check and changes nothing. It exits 1 when a check is `✗`, and 0 otherwise, even when
`wf scaffold` has something to fix. See [Run the stack](#run-the-stack).

### `wf scaffold`

Fixes what the checks find, starts the stack, and runs the checks that need it. It stops before
changing anything when a check needs fixing by hand. See [Run the stack](#run-the-stack).

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

`wf version` prints the version. `wf help` lists the commands, those of the installed components
included. `--json` gives the list in a form an agent can read.

### Exit codes

| Code | Means |
|---|---|
| 0 | done |
| 1 | a check failed, or something went wrong |
| 2 | the command was used wrongly: a typo, a missing flag, a missing setting in `.wf.yml` |
| 130 | you pressed Ctrl-C in `wf up <service>` |

wf asks a question only when it runs in a terminal. In a script or CI it fails instead, and names
the flag to pass.

## Where everything lives

```
.wf.yml                              your settings
.wf.lock                             what wf sync last wrote; never edited
.wf.sum                              what the pinned wf's download must look like; written by wf lock
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
| `pins wf vX, but this is wf vY` | You ran with `WF_NO_LAUNCH=1`, or a wf older than the launcher. Run without `WF_NO_LAUNCH`, or install a newer wf |
| `.wf.sum holds no fingerprint of …` | Run `wf lock` and commit `.wf.sum` |
| `refusing wf X: the download of … is …, but .wf.sum records …` | The download is not what your team recorded. Nothing ran. Don't work around it: tell whoever looks after wf in your project |
| `refusing: the release now gives … the fingerprint …` | The release changed after `.wf.sum` recorded it. `.wf.sum` was left alone. Same as above |
| `.wf.sum: line N is not …`, `… is listed twice` | Restore `.wf.sum` from git, then run `wf lock` |
| `pins wf X, which has no release` | Fix `wf:` in `.wf.yml`: there is no such release |
| `wf has no release for <platform>` | wf runs on macOS and Linux only |
| `was started as wf X, but is wf Y` | `WF_HANDOVER_VERSION` is set in your shell; unset it. A wf from before the launcher (0.1.0-alpha.1) leaves it set for what it runs, so pin a newer one |
| `running wf X from <path>: …` | The cached copy is broken. Delete the folder that holds it, and run the command again to download it anew |
| `skipped … (not written by wf)` | A file of your own is at a path wf writes to. Move it, or run `wf sync --force` to replace it |
| `kept … (no longer generated, but edited by hand)` | wf no longer writes this file, and someone edited it. Delete it yourself if you don't need it |
| `a template takes no project part` | Move that text into `.agents/project/skills/domain-specs/SKILL.md` and delete the file |
| `.wf.yml: pins no wf version` | Add `wf:` with the version `wf version` prints, such as `wf: 0.1.0` |
| `.wf.yml: names no forge`, `forge: … is not one wf has a base for` | Set `forge:` to a forge wf knows: `gitlab` |
| `.wf.yml: gate[0] (lint) needs …` | Give that step what it names: `name:`, `paths:` as a list, `run:` |
| `.wf.yml: doctor[0] (…) needs …`, `has a … that …` | Give that check `name:` and `run:`; `fix:` is a command, and `when:` can only be `up` |
| `has a run: that YAML does not read as text` | Put the command in quotes, such as `run: "false"` |
| `N problems to fix by hand` (doctor), `… so nothing was changed` (scaffold) | Fix each `✗` line `wf doctor` prints, then run the command again |
| `… still fails after its fix ran` | The check's `fix:` does not fix it. Run the fix by hand to see why, or correct it in `.wf.yml` |
| `… fails with the stack up, and it has no fix: to run` | Look at that check's output and the stack's logs: `docker compose logs` |
| `no .wf.yml in …: run wf in the project's root` | Run wf where `.wf.yml` is. `wf up` and `wf down` run there too, so compose uses the project's own compose file |
| `docker compose …: exec: "docker": executable file not found` | Install Docker with its compose plugin, and make sure `docker` is on your `PATH` |
| `docker compose …: exit status N` | docker printed why above. Is Docker running, and is there a compose file at the project's root? |
| `.wf.yml: worktree.copy …`, `worktree.skip …` | Make it a list of globs, such as `[".env*"]`, and check each pattern |
| `.wf.yml: components.<name> is not a component wf knows` | Run the wf version that added it: `.wf.yml` pins it |
| `<component> and another part of the base both have <file>` | Two components clash; add only one of them, and report it |
| `the <component> component is not installed` | Run `wf add <component>` |
| `.wf.yml: components.<name>.<option> …` | Give that answer one the component accepts, as `wf add <name> -h` lists |
| `… is installed already` | Change its answers under `components:` in `.wf.yml`, then run `wf sync` |
| `wf add can only add to components: written out in full` | `components:` comes from a YAML alias (`*name`) or a merge key (`<<:`). Write the components out under `components:` itself, then run `wf add` again |
| `… uses <key>, which .wf.yml does not set` | Add the key it names to `.wf.yml` |
| `has more than one branch` (worktree) | Delete the branch you don't want, or work in the one you do |

## Licence

[MIT](LICENSE)
