# The workflow

The operating manual. This is the file you read once and come back to; it describes the repo as
it is, not as it is planned. If something here disagrees with the code, the file is wrong and
fixing it is part of the change that made it wrong.

Its neighbours, and what each one is for:

| File | What it is |
|---|---|
| `AGENTS.md` | the rules every assistant and contributor reads first |
| `.workflow/specs/<domain>/SPEC.md` | what is true of a domain **now** — state machines, money rules, retry semantics, what is deliberately not built. Edited in place, keeps no history |
| `.workflow/decisions/` | why the system is the way it is, and what we turned down. Numbered, never edited, accepted in an `[ADR]` Issue before the code that relies on them. `README.md` there is the index |
| `.workflow/plans/` | plans in flight, one directory each, `<DD-MM-YYYY>-<slug>/PLAN.md` |
| `.workflow/plans/archive/` | plans whose last Issue has settled |

---

## 1. The life of a change

Every change, a one-file fix included, goes the same way. The `workflow` skill holds the rules;
this is the shape of them.

1. **An Issue.** It exists before the first edit, opens with `## In plain words`, and says what
   done looks like and how to check it. Work too big for one outcome is split into Issues under a
   parent.
2. **A plan, when the work needs one.** Drafted in its own worktree (`wf worktree plan <slug>`),
   read by `/plan-check`, accepted by the developer, merged on its own. Its Issues start after it.
3. **A worktree.** `wf worktree <issue> <slug>` gives the Issue its own checkout and branch, so
   several Issues can be in progress at once without sharing a tree.
4. **The work, uncommitted.** Nothing is committed or pushed until the developer has read it.
5. **The check.** `/pre-mr-check` runs the gate steps the changed paths call for, then the
   `code-reviewer` subagent reads the change in a fresh context. It reports; the developer decides.
6. **Go.** On the developer's go: commit, push, open the MR as a Draft with `Closes #n`, and take it
   out of Draft when the Issue's work is done.
7. **Settled in the same MR.** The domain's spec matches what changed, a decision that constrains
   future code has an accepted record behind it, and a plan whose last Issue this was is archived.
8. **Merged.** The Issue closes with it.

The one way around all of it is a hotfix: a commit starting `Hotfix:`, made only when the
developer's own request starts with `Hotfix:`.

---

## 2. Reference

### Commands

| Command | What it does |
|---|---|
| `wf init [--forge <forge>]` | start using wf: `.wf.yml`, `.agents/base`, and every assistant's files |
| `wf add [<component>]` | list the components wf carries, or add one: its part of `.agents/base`, its files, its answers in `.wf.yml`; `-h` after its name lists its options |
| `wf <component> <command>` | run a command of an installed component; `wf <component> -h` lists them |
| `wf sync` | write every assistant's files from `.agents/base` and `.agents/project` |
| `wf sync --check` | write nothing; fail, naming the file, when a generated file or `.agents/base` differs from what this version of wf writes |
| `wf show <skill> [--base]` | print a skill as an assistant reads it, or wf's part alone |
| `wf worktree <issue> <slug>` | give an Issue its own worktree in `.claude/worktrees/`, or find the one it has; the path is printed last |
| `wf worktree plan <slug>` | the same for a plan being drafted or revised |
| `wf spec-coverage` | every spec's `[ADR-NNNN]` resolves to an accepted record, every record is cited and indexed, every plan is named by its date |
| `wf version` | the version of wf running |
| `wf help [--json]` | every command; `--json` for agents |

### Slash commands

Written by `wf sync` for every assistant from `.agents/`: `.claude/commands/`,
`.opencode/commands/`, and `.github/prompts/` as `<name>.prompt.md`.

| Command | What it does |
|---|---|
| `/plan-check <plan-dir>` | Hands a plan under `.workflow/plans/` to the `plan-reviewer` subagent, which reads it against the repository as it is now: contradictions, wrong assumptions about the code, work with no acceptance checks, a decision that should be a record. Verdict `READY TO IMPLEMENT` or `NOT READY`. The developer accepts the plan; a clean verdict is evidence, not permission |
| `/pre-mr-check` | Run when the work is finished and before anything is committed. Works out which gate steps the changed paths call for, runs every one that applies even after a failure, then hands the diff to the `code-reviewer` subagent. Out comes a gate table, severity-ranked findings and a verdict: `READY FOR REVIEW` or `NOT READY`. It changes nothing |

### Where everything lives

```
.wf.yml                            the project's choices: wf's version, the forge, worktree copies, gate steps, components
.wf.lock                           what wf sync last wrote, with a hash of each file; never edited
.agents/base/                      wf's part of every instruction, skill, command and subagent; never edited
.agents/project/                   the project's part, in the same layout; wins where the two disagree
.workflow/GUIDE.md                 this file
.workflow/SPEC-TEMPLATE.md         what a domain's first spec is copied from
.workflow/DECISION-TEMPLATE.md     what a record is copied from
.workflow/specs/<domain>/SPEC.md   what is true of a domain now
.workflow/decisions/NNNN-<slug>.md why, and what was turned down
.workflow/decisions/README.md      the index of every record
.workflow/plans/<date>-<slug>/     a plan in flight
```

---

## 3. What is enforced, and by what

**Instructions** bind whoever reads them. **A check** binds whoever runs it: `wf sync --check` and
`wf spec-coverage` are gate steps of `/pre-mr-check`, so they run before every push the workflow
offers a check for — and they bind everyone, people and every assistant alike, only where the
project's CI runs them too.

| Rule | Checked by | What it catches |
|---|---|---|
| A generated file is never edited by hand | `wf sync --check` | an edit to a copy that the next `wf sync` would silently overwrite |
| `.agents/base` is the base of the pinned wf version | `wf sync --check` | an edit to wf's part, which belongs in `.agents/project` |
| A spec cites only records still in force | `wf spec-coverage` | a record superseded while the spec describing the old behaviour goes on citing it |
| Every accepted record is cited and indexed | `wf spec-coverage` | reasoning no reader of the spec will ever reach, and an index that disagrees with its files |
| A plan is named by the day it started | `wf spec-coverage` | a plan nobody can date, and files loose in `.workflow/plans/` |

### What is not enforced, stated plainly so nobody assumes otherwise

- **Nothing checks that a spec is true.** `wf spec-coverage` proves the links hold and the records
  agree; whether the prose still describes the code is a person's job, done in the MR that made it
  untrue.
- **Nothing runs these checks on its own.** They run when `/pre-mr-check` runs them, or where the
  project's CI does.
- **Nothing stops a commit before the developer's go.** It is the first rule of the `workflow`
  skill, and it binds by being read.
