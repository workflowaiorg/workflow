# Instructions for AI assistants and contributors

## How work is done here

Every change goes through the `workflow` skill, and through the `forge-[[ .forge ]]` skill for how
each step is done on this project's forge. In short: an Issue before the first edit; a plan
accepted before any Issue comes from it; a worktree per Issue; nothing committed or pushed before
the developer has read the work and said go; `/pre-mr-check` offered before every push; the work
settled in the MR that does it. **Activate both skills before editing any file on a request that is
not read-only.**

## Skills

Activate the matching skill **before** working in its area — don't wait until you're stuck.

- `workflow` — before editing project files on any request that is not read-only, before
  committing or pushing anything, and when asked to merge.
- `forge-[[ .forge ]]` — with `workflow`, whenever it sends you to the forge: Issues, labels,
  milestones, merge requests.
- `domain-specs` — when writing or editing a `.workflow/specs/<domain>/SPEC.md`, when deciding
  whether a choice earns a decision record, when writing or superseding one, or when settling an
  Issue.

## Generated files

`AGENTS.md`, `.claude/CLAUDE.md`, `.github/copilot-instructions.md`, `.workflow/GUIDE.md`, the
`.workflow/` templates, and every skill, command and subagent under `.claude/`, `.github/` and
`.opencode/` are written by `wf sync` from `.agents/`. **Never edit one by hand**: `wf sync --check`
fails on it. The project's own rules go in `.agents/project/`, in the same layout as
`.agents/base/`, and win where the two disagree; `.agents/base/` is wf's and is never edited.
Run `wf sync` after editing `.agents/project/`.

## Conventions

- A code comment states its reason itself and never cites an Issue — the reader should not have to
  leave the code to understand it. Issue links belong in the MR description.
- Do not create documentation files unless explicitly requested.
- **Every plan lives at `.workflow/plans/<DD-MM-YYYY>-<short-slug>/PLAN.md`** — the date the plan
  was started, then a two-to-four-word kebab-case slug naming the work. One directory per plan;
  anything else the plan needs sits beside its `PLAN.md`, never loose in `.workflow/plans/`.
- **A plan is not the record of state.** Two files outlive it: `.workflow/specs/<domain>/SPEC.md`
  says what is true now and is edited in place; `.workflow/decisions/NNNN-<slug>.md` says why, and
  is never edited. A record is accepted in its own `[ADR]` Issue **before** the code that relies
  on it — most decisions never earn one. See the `domain-specs` skill.
- **Every Issue settles in the MR that does the work** — the domain's `SPEC.md` is updated to match
  what changed, and a plan whose last Issue this was moves to `.workflow/plans/archive/`. An Issue
  with no behaviour change settles in one line in the MR description. See the `workflow` skill.
