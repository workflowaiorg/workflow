---
description: "Runs the gate steps the changed paths call for, then has the code-reviewer subagent read the change in a fresh context, and reports one verdict. Read-only: it commits nothing, pushes nothing and writes nothing."
---

Check the work in the tree before the developer reads it. **This command changes nothing**: no file,
no commit, no push, no MR, no label, no comment. It produces a report, and the developer decides what
happens next.

## 0. The range

`git diff $(git merge-base origin/main HEAD)` — the working tree against the point the branch left
`main`, so both committed and uncommitted work is in scope and nothing merged upstream since is,
plus `git status --porcelain` for files a diff cannot see. A new file with no test is exactly what
this catches. Not local `main`: nothing keeps it current.

**Everything here runs in the Issue's — or the plan's — worktree** (the `workflow` skill, *A worktree
per Issue*) — the range, every gate step, and the path the reviewer is given. A tool whose session
sits at the repo root runs each command as `cd <worktree> && …`; the root checkout is not the
change.

Skip only when there is genuinely nothing to speak to: an empty range, or a change that matches no
gate row *and* is one sentence of prose — a typo, a reworded line. Say so in one line and stop.
Everything else runs, skills and plans included: they have gate rows of their own below, and a
generated file that was never re-synced is exactly the kind of thing nobody notices by reading.

## 1. Gate

Work out which steps apply from the paths the range touches, then **run every step that applies,
even after one fails**, so one report shows every problem.

| Step | Changed | Run |
|---|---|---|
| generated files | `.wf.yml`, `.agents/**`, or any file wf generates | `wf sync --check` |
| specs and records | `.workflow/specs/**`, `.workflow/decisions/**`, `.workflow/plans/**` | `wf spec-coverage` |
[[- range gate ]]
| [[ .Name ]] | [[ range $i, $path := .Paths ]][[ if $i ]], [[ end ]]`[[ $path ]]`[[ end ]] | `[[ .Run ]]` |
[[- end ]]

What the paths decide is which steps are ⚪, never whether a part of the project gets checked at
all — **a changed file that matches no row is itself worth saying out loud**, because an all-⚪ table
must mean "nothing applies", never "nobody looked".

Prefer the command the project defines — a gate row of its own, a make target, an npm script —
over a generic equivalent. Report each step as:

- ✅ **pass**
- ❌ **fail**, with the part of the output that says why
- ⚪ **not applicable** — the range does not touch it. This is not a failure.

A step that runs a generator fails when running it *changes something* — which means the change was
made by hand and never regenerated. Name the files it rewrote; "something moved" is not a finding
anyone can act on. That the files are uncommitted is not a failure: nothing is committed at this
point, by design.

## 2. Review

Launch the **`code-reviewer`** subagent — with the Task tool in Claude Code, as `@code-reviewer` in
opencode, as the `code-reviewer` agent in Copilot. Its fresh context is the point: it reads the
change rather than the conversation that produced it, and it reports rather than fixes.

Tell it:

- the worktree's path and the range (`git diff $(git merge-base origin/main HEAD)` there, and **the
  untracked files by name** — a diff does not show them, and a new file is where a missing test
  hides);
- the Issue this work belongs to, by number, so it can read what was promised;
- the gate results, so it does not repeat them;
- that nothing is committed yet, so it does not look for commit messages or an MR.

Do not review the change yourself instead. You wrote it — that is precisely why something else reads
it.

## 3. Report

1. The gate table, one row per step.
2. The reviewer's findings, blocking first, each with its `file:line` — and **its own verdict and
   rationale, in its words**. `Approve`, `Approve with comments` or `Request changes` is its
   judgement of the change; never drop it, summarise it away, or soften it.
3. Then your own line, which is arithmetic rather than judgement:
   - **`READY FOR REVIEW`** — every gate step ✅ or ⚪, and no 🔴 finding.
   - **`NOT READY`** — followed by an ordered checklist of what to fix.

The two answer different questions and can differ in wording without disagreeing: the reviewer says
what it thinks of the change, this line says whether anything blocks. `Approve with comments` with
a green gate is `READY FOR REVIEW`; `Request changes` normally carries at least one 🔴, which makes it
`NOT READY`.
If the reviewer requests changes while raising nothing blocking, say so plainly — that mismatch is
worth the developer's attention, not a silent resolution.

Then stop. Offer the developer the commit message and the MR description you *would* write, and wait
for their word — the `workflow` skill's *Branch and MR* is what happens next, and nothing in it
starts without them.
