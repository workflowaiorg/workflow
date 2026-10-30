---
name: workflow
description: "Runs every implementation task through an Issue — intake before the first edit, the plan a plan-derived Issue waits on, the milestone an Issue of a stage belongs to, the labels that show where a task is, the check and the developer's go that stand between finished work and a commit, branch and MR shape, what a discovered blocker does to the task in hand, how commits are written and how an MR is merged, and the one documented way around all of it. Activate before editing project files on any request that is not read-only, before committing or pushing anything, and when asked to merge."
---

# Skill: The workflow

The forge — GitLab, GitHub — holds who is doing what, how far it got and whether it was accepted.
The repository holds the implementation, the specs and the engineering rules. This skill is the
seam between the two, and it holds on any forge. How each step is done on this project's forge —
the CLI, the label syntax, the traps — is the `forge-[[ .forge ]]` skill; activate it with this one.
*How* to build the thing lives in `.workflow/GUIDE.md` and the skills of each area; this file does
not restate it.

An MR here is the forge's unit of review: a merge request on GitLab, a pull request on GitHub.

---

## When this skill applies

Activate it when you are:

- Asked to implement, change, fix or remove anything in the repository — a one-file fix included
- Handed an Issue, or a request that should have one behind it
- Discovering, mid-task, that something outside the task must change first
- Finished with the work and about to commit, push, open an MR or answer a review on one
- Asked to merge an MR

**Not this skill:** explaining, inspecting, reviewing someone else's MR — read-only work needs no
Issue and writes nothing to the forge. Drafting a plan needs no Issue either, but it is written in
a worktree of its own and lands through an MR: see
[Plans](#plans-have-a-worktree-and-an-mr-of-their-own). Not this skill either: what to build (the
skills of each area).

---

## Intake: an Issue before the first edit

1. **Given an Issue**, read its description, its discussion and its linked MRs before touching a
   file. **Given a plain request**, search the project's issues for one that already covers it and
   reuse it without widening its scope; if there is none, create one *before* the first edit and
   put the request in it in the requester's own words.
2. **Every Issue opens with `## In plain words`:** what is missing, why it matters and what
   changes, in a few short sentences a 10-year-old could follow — everyday words, with no file
   paths, code names, labels, commands or project jargon. If it cannot be said that simply, the
   Issue is not understood well enough to start. Every Issue you create has it — sub-Issues,
   blockers and `[ADR]` Issues included — and one handed to you without it gets it added before
   work starts. Below it, an executable Issue says, with
   real values: the original request; the observable result; scope and non-goals; the expected
   files; the acceptance checks and the commands that run them; and, when the work comes from a
   plan, the plan path under `.workflow/plans/` and the section. It carries one `type` and one
   `area` label from the moment it is created, and the status `todo`.
3. **Settle the Milestone field** of every Issue you create or are handed, by
   [Milestone](#milestone-an-issue-of-a-stage-belongs-to-it), before any work starts: the
   milestone of the stage it belongs to, or none for a one-off. A stage named only in the
   description does not count.
4. **One Issue is one outcome.** Work with several independent outcomes is split into linked
   Issues before anything is edited, all linked from one parent Issue.
5. A request outside any agreed scope is a question to the owner, not an edit. Ask, and write
   nothing until the answer. The same holds for a concern or question the developer raises about a
   plan, a design or the code: answer it in chat with a recommendation and its trade-off, and
   change nothing until they agree — even where you are allowed to edit, as in a plan's worktree. A
   rewritten file presents a decision as already made, and the decision is theirs.
6. No access to the forge, or no Issue you are allowed to create: report the exact blocker and
   stop. Do not code first and promise the Issue later.

**An Issue that comes from a plan waits for that plan to be accepted.** Run `/plan-check <plan-dir>`:
the `plan-reviewer` subagent reads the plan in a fresh context and returns `READY TO IMPLEMENT` or
`NOT READY`. `NOT READY` means the plan is fixed first, and what changes in it is the developer's
call — it is the agreement, not your draft. A clean verdict is evidence, not permission: **the
developer accepts the plan, and until they have, you create no Issue, no milestone and no branch from
it.** Drafting and reviewing a plan needs no Issue of its own, and no Issue of the plan starts work
until the plan's own MR has merged — see [Plans](#plans-have-a-worktree-and-an-mr-of-their-own).

---

## Milestone: an Issue of a stage belongs to it

The test for every Issue: **does it move some milestone's completion criteria?**

- **Yes** — its Milestone field is set to that milestone, the stage whose result it delivers.
- **No** — it is a **one-off**, and it has no milestone. A one-off belongs to no stage: a workflow
  or skill fix, a tooling bug, something found in passing. Never create or reopen a milestone for
  one — a milestone that exists to hold one task says nothing about any stage.

No milestone is a decision, not an omission: it means the test was run and the answer was no. This
is a rule of this skill that the agent follows and checks; the forge does not enforce it, and
nothing automates it.

**Milestones are buckets, not steps.** They have no order, nothing runs them one after another,
and several are normally open and worked on at the same time. A milestone says what has to be
true when its group of work is done — never when that work happens relative to another
milestone. Order lives in the dependencies written inside each Issue; a milestone never implies
one.

1. **Reuse first.** Look through the project's existing milestones for the one whose stage of the
   plan and expected result the Issue serves, and assign it. Never create a duplicate, and never
   pick an arbitrary milestone just so the field is not empty.
2. **None fits, and the stage is already agreed:** create one milestone for that stage with a
   plain name; its goal and boundaries; its completion criteria; and links to the plan and the
   parent Issue where they exist. Leave start and due dates empty unless the owner has given them —
   dates are never invented. **The name carries no letter or number in front of it:** a number in
   front reads as a step in a sequence, which is exactly what a milestone is not.
3. **Ask the owner and do not start the task** when creating a milestone would widen the agreed
   scope, when it is unclear which stage the Issue belongs to, or when you lack the rights to
   create or assign one. Write the question in the Issue.
4. **The milestone list has one owner.** Anyone may propose a milestone; one person creates it.
   The forge does not enforce this, so it is a rule people keep, not a permission. Two people
   creating milestones independently puts the same work in two stages, and then the board tells
   nobody anything.
5. **One stage, one milestone.** Every Issue of a stage shares it; a small task never gets a
   milestone of its own.
6. **Sub-Issues and blockers take the same test.** Usually they share the milestone of the Issue
   they came from — or none, when that Issue is a one-off — but check it against what the new
   Issue delivers; if it belongs to another stage, assign that one and keep the dependency link
   between the two Issues explicit.
7. **Before the status `in progress`,** check again that the milestone is settled — set and still
   correct, or deliberately none for a one-off. A closed milestone is not reopened by an agent —
   if it is the only fit for an Issue of a stage, ask the owner.
8. **A milestone settles only the stage.** It does not replace checking the assignee, your
   authority over the area, the dependencies and the acceptance criteria.

This is not a mandate to re-sort existing Issues: fix the milestone of the Issue you are about to
work on, not of the whole board.
A hotfix has no Issue and so no milestone; this section does not change
[Hotfix](#hotfix-the-one-way-around-the-workflow).

---

## How much one task may carry

Keep one Issue to one outcome and split big work into sub-Issues, unless it truly has to land
together — a new endpoint with its handler, its storage, its migration and its tests is one outcome
and lands as one. **From around 13 changed files, the MR description says why the work was not
split.** That is the whole rule. A generated file counts as one with its source in `.agents/`:
a skill and its copies for every assistant are one file. Never drop a test or a contract change
to make a diff look smaller.

**Whatever its size, an MR is complete when it leaves Draft.** Nothing in it is postponed, and no
comment or file exists only to explain a temporary state — "removed together with its last call
site", "built in the next stage" — because such a comment ages into a lie. No dead props, keys or
branches; no plan-tracking or review write-ups among the product changes. The unit is a complete
surface, not a screen: a shared component moves together with everything that uses it. Splitting
work across MRs that each leave a mixed state is how dead files and non-working buttons reach
production. An element that truly cannot be avoided is named in the MR description as its single
exception.

---

## Labels: the board shows where every task is

Three groups of labels, each Issue carrying one of each:

| Group | Values |
|---|---|
| Status | `todo`, `in progress`, `in review`, `blocked` |
| Type | `feature`, `bug`, `chore`, `docs`, `decision` |
| Area | one per part of the project — the project part of this skill lists them |

How a label is spelled and swapped on this forge is the `forge-[[ .forge ]]` skill. A maintainer
creates and changes the labels there, and the repository keeps no copy of the set — a second copy
is one more thing that can quietly stop being true. The board has one column per status and shows
open Issues only: a closed Issue keeps the status it had. An agent only *uses* labels — if one this
skill names is missing, say so in the Issue and tell the maintainer; do not create labels ad hoc.

Type and area are set when the Issue is created, and corrected if they are wrong. Status is set
by whoever moves the task, at these moments:

| Moment | Status |
|---|---|
| Issue created | `todo` |
| Branch and worktree created, work started (milestone checked) | `in progress` |
| MR moved out of Draft | `in review` |
| Blocker found | `blocked`, plus a comment linking the blocker's Issue |
| Blocker resolved | back to `in progress`, after re-checking the milestone |
| MR merged | the Issue closes automatically (see Closing) |

An Issue carries exactly one status. Before setting one, check that it has no other, and repair it
if a hand edit on the board left two.

---

## Branch and MR

- One Issue, one branch, normally one MR. The branch is `<issue>-<short-slug>`, e.g.
  `12-sound-toggle`; a plan's is `plan-<slug>`
  ([Plans](#plans-have-a-worktree-and-an-mr-of-their-own)). Nothing is committed to `main` directly —
  the only exception is a maintainer's hotfix (below). Work is handed to another executor through
  its Issue, never as a working tree someone else has to guess at: see the checkpoint in
  [A discovered blocker](#a-discovered-blocker).
- One active Issue per session — see [A worktree per Issue](#a-worktree-per-issue). Before
  claiming — whether handed an Issue or choosing the next one yourself — check the assignee, the
  latest comments, the dependencies and the Milestone field; an Issue someone else is working on is
  not taken over without a written handoff, and an Issue whose milestone is wrong is not
  started until [Milestone](#milestone-an-issue-of-a-stage-belongs-to-it) is settled.
- **An agent commits nothing and pushes nothing on its own.** The work stays in the working tree —
  uncommitted, unpushed, no branch on the server, no MR, not even a Draft one — until the developer
  has read it and said go. Committing first is already too late: it puts a message and a boundary on
  work nobody has agreed to, and every later change then arrives as a second commit or a rewritten
  history.
- **"Go" is the usual word, not the only one.** Any clear approval of the work in hand counts the
  same — "all good, proceed", "commit it", "agreed". An approval covers only the work it was given
  for; the next step needs its own.
- Once they say go: commit, push — the first push is `git push -u origin <branch>`, since the
  branch starts with no upstream — and open the MR as a Draft. The description carries `Closes #n`
  from the start, plus the result, the scope, and the checks run with their results.

### A worktree per Issue

Work waits uncommitted until the developer's go, so one working tree can hold one Issue's work at a
time. Each Issue therefore gets its own git worktree, and several can be worked on at once, one
session each, while the root checkout stays on `main`. The agent makes it; the developer runs
nothing.

- **Start and resume are one command.** When you start an Issue, before the status `in progress`,
  run `wf worktree <issue> <slug>` and go to the path it prints last. A new Issue gets
  `.claude/worktrees/<issue>-<slug>` on the branch `<issue>-<slug>` from `origin/main`; an Issue
  whose branch is already checked out somewhere gets that checkout back, and one that has only a
  branch gets a worktree on that branch. The gitignored files `.wf.yml` lists under `worktree:` are
  copied in from the root checkout.
- **Then say what the Issue is about.** Once it is started or resumed, before the first edit,
  tell the developer in two or three plain sentences what the work is, so a wrong Issue or a
  misread one is caught before anything is written.
- **Work there.** Claude Code moves the session in with a plain `cd <path>` in the shell — never
  `EnterWorktree`, which turns on a check that refuses any command that may run git when part of it
  is only known at run time — `$(cat file)`, a heredoc — and has no setting to turn it off. After
  the `cd`, every command and every hook runs in the worktree. A tool that cannot move its session
  stays at the repo root and runs everything inside the worktree — `cd <worktree> && …`,
  `git -C <worktree> …` — `/pre-mr-check` included. Nothing stops a command from running in the
  root checkout by mistake: anything the work leaves in the root checkout's `git status` was
  written to the wrong tree. The one exception: when the printed path is the root checkout itself,
  the Issue was started there before it had a worktree — work there in place, and the root stays
  off `main` until that Issue's MR merges.
- **One Issue per session and per working tree** by default. A session that switches Issues leaves
  the first one's work in its worktree, uncommitted, and says in that Issue where it stopped. Two or
  more Issues share one tree only when the developer asks, and they say which Issue's worktree
  carries the others; its MR closes every Issue in it.
- **Clean up** once the MR has merged: `git worktree remove <worktree>` and `git branch -D <branch>`.
  `git worktree remove` refuses while anything is uncommitted — that is the answer, not an obstacle
  to force past.

### Plans have a worktree and an MR of their own

A plan comes before its Issues, so it cannot live in one of theirs; and two sessions planning at
once must not share a folder. So a plan is written the way an Issue is worked — alone, in its own
worktree — and reaches `main` before any of its Issues start, where every one of them finds it.

- **A plan lives at `.workflow/plans/<DD-MM-YYYY>-<short-slug>/PLAN.md`** — the date it was
  started, then a two-to-four-word kebab-case slug naming the work. One directory per plan;
  anything else the plan needs sits beside its `PLAN.md`, never loose in `.workflow/plans/`.
  `wf spec-coverage` fails on any other name.
- **Draft and revise in the plan's worktree.** `wf worktree plan <slug>` makes or finds
  `.claude/worktrees/plan-<slug>` on the branch `plan-<slug>` from `origin/main`, where `<slug>` is
  the plan directory's slug. A new plan and a revision of one already on `main` are both written
  there, never in the root checkout. `/plan-check` reviews it there.
- **Accepted, it lands on its own.** When the developer accepts the plan, create its parent Issue
  (and the milestone, by the rules above). Then, on their go, commit the plan and open its MR: the
  one MR with no `Closes` — it names the parent Issue as `Part of #n`, since the plan starts that
  Issue's work rather than finishing it. It carries the plan and nothing else.
- **Issues start after it merges.** Every Issue of the plan branches from an `origin/main` that
  already holds the plan. The last Issue still archives the plan in its own MR, by
  [Settling](#settling).
- **Clean up once the plan's MR merges**, as for an Issue: `git worktree remove` and
  `git branch -D plan-<slug>`. A later revision starts over with `wf worktree plan <slug>`,
  which then branches fresh from `origin/main` — a kept branch would predate the squash-merge and
  everything since — and lands in an MR of its own, `Part of #parent`.

### When the work is finished

1. **Say what you did.** What changed, what you checked, what you did not, and what you are unsure
   about — before any of it is committed.
2. **Offer the check, in one line:** `/pre-mr-check` runs the gate steps the changed paths call
   for and then has the `code-reviewer` subagent read the change in a fresh context, where it
   reports and never edits. It is the developer's to refuse — but being asked is not optional, and
   **refusing it authorises nothing.** The one time it is not offered is when there is genuinely
   nothing to speak to: an empty tree, or a one-sentence change that matches no gate step. Then say
   so in one line and offer nothing. A skill, a plan or a generated file is never "just docs" —
   those have gate steps of their own.
3. **Wait.** Whatever the answer and whatever the verdict, the work stays in the tree:

| The developer says | What you do |
|---|---|
| No | Report what changed. Commit nothing, push nothing. They will read it themselves, or say what to change |
| Yes → `NOT READY` | Report the gate failures and the 🔴 findings. Commit nothing, push nothing. They decide what gets fixed now |
| Yes → `READY FOR REVIEW` | Report the verdict and the 🟡/🟢 findings, and **propose** the commit message, the MR description and the label. Then wait |
| Go | If an acceptance check of the Issue is still open — a measurement only a person can run, say — the reply opens with one plain line, "#n is not finished: <what is left>", and waits; an approval given after that line covers the open check, and the MR description names it. Never fold it into a 🟡, a trailing note or a `[pending]` line in the MR description. Otherwise: commit, push, open or update the MR; and when the Issue's work is done, move its status from `in progress` to `in review` and take the MR out of Draft |

Nothing reaches the remote or the forge before that go — no commit, no branch, no MR, no
description, no comment, no label. The check is what the developer reads while deciding; it is
never a substitute for their deciding.

**How many rounds there are is the developer's call, never yours.** After every verdict you stop:
report what is left, say what each finding would cost to fix, and wait. You never decide to fix and
re-run on your own — one run may be enough and the developer commits over an open 🟡, or they may
send it back five times. Findings do not have to reach zero for work to be good enough; only the
developer says when it is.

Four lines keep it honest:

- **Unsure counts as a blocker.** You wrote the code, so the tie-break leans to stopping.
- Relay a finding — the reviewer's or your own — only after checking it is real and sized right,
  by the bar in the `code-reviewer`'s "How to report". Noise is dropped, not listed as low priority.
- A finding that is real work outside this Issue's scope is a linked Issue by
  [the blocker table](#a-discovered-blocker), never a quiet fix in this MR.
- A finding the developer dismisses is recorded in the MR thread — once they have approved that
  posting — and not raised again on a re-run.

---

## Review

Move an MR out of Draft only with results for the commit under review; a check that was not run,
or failed, is written down as such, not left out — and that includes `/pre-mr-check` when the
developer declined it. Every finding gets either a fixing commit or a written objection, in its
thread. Fixes are new commits on top — the reviewer cites hashes — never a rewritten history. A new
commit reopens every review thread it touches.

Every push after the first goes through [the same sequence](#when-the-work-is-finished): the fix
stays in the tree, the check is offered, the developer says go. A round of review answers is not an
exception to it.

---

## Commits and merging

A commit message is read by someone who was not there, in `git log`, years from now. It explains
the change on its own:

- A summary line in plain words, under about 72 characters; a blank line; then a list of what
  was done.
- No internal jargon, no tool output, no references to review threads.
- **No AI signatures of any kind** — no `Co-Authored-By` for a tool, no "Generated with", no
  session links. Only the substance of the change.

```
Add a sound on/off switch

- Add a button in the header that mutes and unmutes the sound
- Remember the choice between visits
- Keep the sound on by default for new users
```

Squashing an MR into one commit with a message in this shape keeps `main` readable and is the
preferred way to merge; it is not required. **When asked to merge an MR with more than one commit,
ask whether to squash before merging.** If yes, write the squash message in the shape above; if no,
merge as is. An MR with a single commit has nothing to squash: merge it as is, without asking.

---

## A discovered blocker

When the task in hand needs a change outside its own scope:

| Situation | What you do |
|---|---|
| Something outside the task must change first | Stop editing the original. Leave a checkpoint in its Issue: done and remaining, the branch and its worktree, the checks run, the exact next step, the evidence. Nothing is committed for a checkpoint either — describe the state of the tree in the comment, and ask the developer if the work should be parked on a commit before you leave it. Find or create the blocker's Issue by the intake rules, settle its milestone by [Milestone](#milestone-an-issue-of-a-stage-belongs-to-it) rule 6, link the two both ways, set the original's status to `blocked`. |
| The blocker is in your area and inside the agreed scope | Take it in its own branch and MR through the normal review. Never fold it into the original MR. |
| It belongs to the other side, needs a business decision, or more permissions than you have | Hand it to its owner in the blocker's Issue and stop that path. |
| Debt noticed in passing that the task does not need | Record it in an Issue and leave it alone. Discovering it is not authority to fix it. This means debt in code the MR did not write: duplication or a shortcut in the MR's own diff is fixed in that MR, and a convention that forces it is amended in the same MR — code quality wins, and the convention is what gets fixed. |
| A decision you are about to make would constrain future code | Stop and open an `[ADR]` Issue — type `decision`, the relevant area, the milestone of the Issue it blocks if that has one, and **the record itself as the body**, word for word, below its `## In plain words` and above one line naming what it blocks. One Issue per record. Set the original to `blocked` and link the two. The developer's answer in that thread is what accepts it; the record then lands in its own MR, never folded into this one. The test for whether a decision qualifies, and the file's shape, are in the `domain-specs` skill, *When a decision earns a record* — most do not qualify. |
| The blocker is merged (and deployed, if that was the need) | Come back: update the branch, rerun the checks, set `in progress`, continue from the checkpoint. |

---

## Settling

Every Issue leaves the repo's own record of itself true, **in the same MR as the work** — never in
a follow-up. A separate cleanup pass does not happen; the git log of every project proves it.

1. **The domain's `.workflow/specs/<domain>/SPEC.md`** is updated to match what this MR changed,
   citing any record accepted for it at the point of the claim. If the domain has no spec yet,
   writing its first one is an Issue of its own and not yours to start here — say so in the MR
   description.
2. **Anything the work turned up that would constrain future code** opens an `[ADR]` Issue, by the
   [blocker](#a-discovered-blocker) row above, before this MR merges. Settling never *writes* a
   record: records are accepted in their own Issue, ahead of the code.
3. **If this Issue is the last of its plan's Issues** — the parent closes with it — the plan
   directory moves to `.workflow/plans/archive/<date>-<slug>/` and its header gains
   `Settled: <date>`. A plan with phases still ahead of it stays where it is; a phase closing is
   not the plan closing. **The move belongs in this MR, not after it merges.** You already know
   whether anything of the plan is left, so "wait until it is merged and then tidy up" is the
   follow-up this section exists to prevent — and the plan is no harder to review at its archived
   path, where the diff shows it as a rename.
4. **What this MR deletes, it deletes completely.** Everything that only existed for the deleted
   thing goes too: its functions, its mocks, its config fields and env vars, its tests and test
   data. Dead code that was there before the MR is not touched; it gets its own Issue.

**An Issue with no behaviour change settles in one line in the MR description and touches no
files.** Most of the board is small tasks, and a settle step that is not cheap on those is a
settle step that gets skipped.

What goes in a spec, what stays out, and when a decision earns a record: the `domain-specs` skill.
That skill owns the formats; this one owns when in the Issue's life they are written.

---

## Closing

- An Issue closes **automatically** when its MR merges — the description carries `Closes #n`.
  Nobody closes Issues by hand.
- A parent Issue closes when its last child MR merges, carrying `Closes #parent` too.
- **An MR that must not close an Issue never names it next to a closing word** — "close", "fix",
  "resolve" or "implement" in any form — not even negated, since a forge reads "does not close #n"
  as `close #n`. Write "#n stays open until …" instead, and after the merge check that the Issue is
  still open; if it was closed anyway, reopen it — the one reopen allowed.
- A bug found after merge is a **new** `bug` Issue linked to the original. A closed Issue is never
  reopened.
- Merge is not deployment. If the result can only be seen in production, checking it there is
  part of the new Issue's work, not a reason to keep the old one open.

---

## Hotfix: the one way around the workflow

A hotfix is a commit pushed directly to the target branch. It skips intake, branch, MR and review.

- The commit message starts with `Hotfix:`, then says in plain words what was broken and what
  the fix does.
- An agent does a hotfix **only when the developer's request itself starts with `Hotfix:`**. An
  agent never decides on its own that something is a hotfix.
- Where `main` is protected, an agent that cannot push prepares the commit on the developer's
  checkout and hands the push to them; it does not open an MR instead.
- If a push to `main` deploys anything with no review, say so to the developer before the push.
- No Issue is written afterwards. The `Hotfix:` prefix is the record —
  `git log --format='%h %s' | grep -E '^[0-9a-f]+ Hotfix:'` lists every one with its subject.
  Follow-up work is a normal Issue.

---

## Definition of done

- [ ] The MR links an Issue that existed before the first edit and carries `Closes #n`; a plan's
      own MR instead carries `Part of #parent`, naming the Issue created when the plan was accepted.
      The linked Issue opens with `## In plain words` a 10-year-old could follow, and holds the
      request in the requester's own words with its type, area and status labels
- [ ] The developer read the work before anything was committed or pushed, and where it came from a
      plan, they had accepted that plan before the Issue existed
- [ ] `/pre-mr-check` was offered before each push; where it ran, the gate was green or ⚪ and every
      🔴 finding has a fixing commit or a written objection; where it was declined, or not offered
      because there was nothing to check, the description says so
- [ ] The Issue's milestone was settled before `in progress` — its stage's, or none for a one-off —
      and every sub-Issue or blocker Issue it spawned took the same test
- [ ] From around 13 changed files (generated files counted as one with their source), the
      description says why the work was not split
- [ ] Nothing in the MR is postponed or half-migrated, and no comment or file explains a temporary
      state
- [ ] The MR description states result, scope, and the checks with their results
- [ ] Every reviewer finding has a fixing commit or a written objection in its thread
- [ ] Every commit message is summary + list in plain words, with no AI signature
- [ ] Anything discovered on the way lives in its own linked Issue, not in this MR
- [ ] The work is [settled](#settling) in this MR — the domain's `SPEC.md` matches what changed, any
      constraining decision has an accepted record behind it, and a settled plan is archived; or the
      description says in one line that nothing changed
