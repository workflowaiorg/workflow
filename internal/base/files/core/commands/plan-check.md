---
description: "Has the plan-reviewer subagent read a plan in a fresh context before anything is built from it, and reports one verdict — READY TO IMPLEMENT or NOT READY. Read-only: it changes neither the plan nor the forge."
argument-hint: <plan directory under .workflow/plans/>
---

Review the plan at `$ARGUMENTS` before any Issue, milestone or branch comes out of it. **This command
changes nothing** — not the plan, not the forge. It produces a report for the developer, who is the
only one who accepts a plan.

## 0. Which plan

`$ARGUMENTS` is a directory under `.workflow/plans/`, or enough of its name to find one. Find the
copy under review the way `wf worktree plan` finds a plan — by its branch, not its path:

- **A checkout on `plan-<slug>`**, wherever it is — the `worktree` line above
  `branch refs/heads/plan-<slug>` in `git worktree list --porcelain` — holds the draft or revision.
  Give the reviewer that directory's full path.
- **No such checkout, and the plan is on `origin/main`:** it is reviewed as it is there, never from
  a checkout that may be behind it — give the reviewer `origin/main:.workflow/plans/<dir>`.
- **No plan branch and nothing on `origin/main`, but an untracked copy sits in some checkout:** the
  draft predates its worktree. Do not review it where it lies; say so, and that it moves into
  `wf worktree plan <slug>` first.

If it names no plan, or several, say so and stop rather than guessing.

There is no gate here — nothing compiles — so this is the review and the verdict.

## 1. Review

Launch the **`plan-reviewer`** subagent — with the Task tool in Claude Code, as `@plan-reviewer` in
opencode, as the `plan-reviewer` agent in Copilot. It reports and changes nothing, least of all the
plan. Tell it the plan's location from §0 and nothing about the conversation that produced the plan: an
author's reasoning is the one thing a reviewer must not be given.

Do not review the plan yourself instead.

## 2. Report

1. The findings, blocking first, each citing the plan's own headings and the code it was compared
   against.
2. A verdict on its own line:
   - **`READY TO IMPLEMENT`** — nothing blocking remains.
   - **`NOT READY`** — followed by an ordered list of what the plan still owes.

Then stop, and put it to the developer:

- **`NOT READY`** — the plan is the agreement, so what changes in it is theirs to decide. Propose the
  edits; make none unasked. Run this again after.
- **`READY TO IMPLEMENT`** — ask them to accept it. A clean verdict is evidence, not permission.
  Until they have accepted, create no Issue, no milestone and no branch from this plan (the
  `workflow` skill, *Intake*).
