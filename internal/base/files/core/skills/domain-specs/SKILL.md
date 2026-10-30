---
name: domain-specs
description: "Covers the two artifacts that outlive a plan — a domain's SPEC.md saying what is true now, and the numbered decision records saying why. What belongs in each, the test for when a decision earns a record, how a record is accepted before the code it constrains, and what settling an Issue leaves behind."
---

# Skill: Domain specs and decision records

A plan says what we are about to do. It goes stale the day it is implemented, and every project's
git log proves it. Two artifacts outlive it:

| File | Answers | Changes? |
|---|---|---|
| `.workflow/specs/<domain>/SPEC.md` | what is true **now** | edited in place, keeps no history |
| `.workflow/decisions/NNNN-<slug>.md` | why, and what we turned down | never — a change gets a new record |

They need each other. A spec is edited in place, so it always describes the present — and so it
quietly erases the fact that another option was ever on the table. A record is the mirror image:
it remembers everything and tells you nothing about today.

**This skill owns the formats.** *When* in an Issue's life you write them is the `workflow` skill's
*Settling* section. The two must not disagree; if they do, this file is the authority on shape and
`workflow` is the authority on timing.

---

## When this skill applies

- Writing or editing any `.workflow/specs/<domain>/SPEC.md`
- Deciding whether something you just chose earns a decision record
- Writing a record, or superseding one
- Writing a plan's `## Spec changes` section
- Settling an Issue, alongside the `workflow` skill

---

## The one rule a spec lives by

> **If a machine already checks it, the spec does not repeat it.**

Response shapes, routes, schemas: whatever the project already pins in a file CI checks — a
contract, a manifest, a migration — belongs to that file. A spec that restates it adds a copy that
can rot, and rots first.

So a spec holds what is currently homeless:

| Belongs in a spec | Belongs elsewhere |
|---|---|
| State machines, and which transitions cannot happen | Field names and shapes → the contract CI checks |
| Money rules, rounding, what a refund covers | Route paths and methods → wherever routes are registered and checked |
| Retry, ack and idempotency semantics | Request and response examples → the contract |
| Invariants two services depend on | Struct definitions → the code |
| What is deliberately **not** built, and why not | How to run the thing → `.workflow/GUIDE.md` |
| Links to the records behind any of the above | What we are about to change → a `PLAN.md` |

The last row on the left is the one people skip and the one that saves the most time. *"There is
no partial refund; a failed order refunds in full"* stops someone building the partial-refund
path that was already considered and rejected.

### Shape of a SPEC.md

Start from `.workflow/SPEC-TEMPLATE.md`. Prose, present tense, second person never. Cite a record
at the point of the claim it supports:

```markdown
### Cancelling an order

An order can be cancelled until it ships, and a cancelled order is refunded in full. Two
cancellations racing each other both succeed; the second refund finds nothing left to refund.
→ [ADR-0007](../../decisions/0007-cancellation-refunds-in-full.md)
```

**A citation is a link, so a reader can follow it in one click**, and the link text is
`[ADR-NNNN]` exactly — that is what `wf spec-coverage` looks for, and a renamed label stops
being found. The target is the record's path relative to the spec, which from
`.workflow/specs/<domain>/` is always `../../decisions/`.

Put a long citation on its own line rather than letting it run past the wrap.

### Writing a domain's first spec

A domain that has no spec yet gets one in an Issue of its own. Read its code, the finished plans
that built it, the skills that govern it, and whatever contracts already pin its surface. Then
write down what is true today — not what should be true. Where the two differ, the spec says what
the code does, and the gap becomes its own Issue.

The domain's old decisions come across with it. They are already built and shipped, so there is no
code waiting on them and nothing to block: writing them down now is transcription, not deciding.
Each still gets [its own `[ADR]` Issue](#how-a-record-is-accepted), carrying the record word for
word, accepted or rejected on its own. **Never one Issue listing a domain's decisions** — a
reviewer cannot accept half a wall of text, and the file that then gets written is not the one they
read.

What the backfill changes is only where the records land: with the spec, in this Issue's MR,
because the code they describe is already there.

---

## When a decision earns a record

Most do not. A normal feature is already covered by its spec and its contract; a record there is a
file nobody will ever open.

The test:

> Could a future developer — or an agent — **undo this by accident**, thinking they had found a bug?

Two examples:

- *"A retried message that arrives late is never applied"* — **yes, write one.** Someone will
  notice the late message being dropped, call it a bug, and fix it. It was weighed and accepted.
- *"The plan's phases stay separate because each one compiles"* — **no.** Scaffolding for one
  plan. It dies with the plan.

Expect zero to three per plan, weighted heavily toward zero. A record is for a choice that
**constrains future code**, not for every judgement made along the way.

A record costs an Issue and a round of review, so the temptation is to skip one rather than open
the Issue. The test is the guard against that, and it cuts both ways: a decision that passes is
worth the Issue precisely because someone will otherwise undo it by accident. A decision that
fails was never going to earn a file.

A plan's own `## Decisions taken` section does not change. That is where thinking happens, freely
and in context.

---

## How a record is accepted

**Before the implementation it constrains, in an Issue of its own.** Not afterwards in the MR that
built it — by then the decision is load-bearing and the record is a formality nobody weighed.

**1. The ADR Issue.** Title `[ADR] <the decision in one line>`. Type `decision` and the relevant
area. The milestone of the Issue it blocks, if that has one.

**The body is the record itself, word for word** — the four sections of
`.workflow/DECISION-TEMPLATE.md` and nothing else, above one line naming what waits on it:

```markdown
**Blocks:** #41. Accepting this commits it in as the record file, as written below.

## Decision
## Alternatives considered
## Consequences
## Undoing this by accident
```

Accepting therefore means *"commit this file as written"*, and amending means editing a line in
the thread. **Anything the developer reads that is not the record is a second draft nobody
reviews** — no framing, no summary of the argument, no options to pick between. Keep it to about
a dozen lines; a record that needs more is usually two decisions.

One Issue holds one record. Four decisions are four Issues, so each is accepted, rejected or
amended on its own and closes on its own — **including a backfill**, where it is tempting to put a
domain's decisions in one thread and where the wall of text costs the reviewer most.

**2. The discussion is the gate.** The developer accepts, rejects or amends it in the thread.
**Never write the file ahead of that answer, and never decide on your own that a record is
accepted.**

**3. The record lands in its own MR** — one file, plus its row in `.workflow/decisions/README.md`.
It carries `Closes #<adr-issue>`. The exception is a backfill: those records constrain code that
already exists, so there is nothing for them to land ahead of, and they ride with the domain's spec
in that Issue's MR.

The spec still cites a record whose code is not built yet — `wf spec-coverage` fails on a record
no spec links to. At the claim it will change, add one line: **Decided, not yet built**, what will
be true, the Issue that builds it, and the link. That is true today, so the spec stays in the
present tense. The Issue that builds it replaces the line with the behaviour, in its own MR.

**4. The dependent Issue waits** at `blocked` with a comment linking the ADR Issue, and returns to
`in progress` when the record merges. That is the `workflow` skill's *A discovered blocker* table
applied unchanged.

**Found mid-implementation?** Same path, entered through the blocker rule: stop editing, leave the
checkpoint in the Issue, open the ADR Issue, block the original. A decision important enough to
record is important enough to stop for.

### The record file

Start from `.workflow/DECISION-TEMPLATE.md`.

```markdown
---
id: 0007
title: Cancelling an order refunds it in full
date: 2026-09-14
status: accepted
domain: orders
spec: .workflow/specs/orders/SPEC.md
supersedes: —
superseded_by: —
issue: 42
---
```

`status` is `accepted` or `superseded`. `domain` is `none` for a process-level decision, and `spec`
is `none` when no spec cites it. `supersedes` is the `NNNN` of the record this one replaces, or `—`.

**There is no `proposed` status.** The Issue is the proposed state; a rejected decision leaves a
closed Issue and no file, which is a better record of a road not taken than a deleted one.

**IDs are flat and numbered, not nested under a domain.** The records that matter most cross
domains — how the code is split, how a release is gated. Nesting forces a false choice for exactly
those. `domain:` gives filtering without it, and a flat integer gives an ID you can say out loud in
review. Two branches can both grab `0031`; renumber at merge, it takes half a minute.

### Superseding

A record is **never edited**. A decision that changes gets a new record and a new ADR Issue —
changing your mind is itself a decision. The new record says `supersedes:` the old number from
the moment it is written. The single allowed edit to an existing record is marking it replaced —
`status: superseded` and `superseded_by:` the new number — and at that moment the spec's
`→ [ADR-NNNN]` must move to the new number. The two records name each other, and so do the index
rows: the old row's `Superseded by` and the new row's `Supersedes`. A record with no `supersedes:`
line means `—`.

The new record lands before the code that builds it, so for a while the spec describes behaviour
whose record is already superseded. The claim keeps describing today, and carries the **Decided,
not yet built** line linking the new record; the old record is one click further, through its
`supersedes:`.

---

## The `## Spec changes` section

Written into a `PLAN.md` while the work is being planned, when the context is already in your
head. It makes settling mechanical instead of a re-read of the whole diff.

```markdown
## Spec changes

`.workflow/specs/orders/SPEC.md`

### MODIFIED: Cancelling an order
Cancellable until it ships, refunded in full.
*(was: cancellable for one hour after it was placed, refunded less a fee)*  → ADR-0007

### REMOVED: The cancellation fee
Dropped outright, along with its setting.
```

`ADDED` / `MODIFIED` / `REMOVED`, one heading each. **The `(was: …)` line is the point of the whole
format** — a reviewer sees what is changing without diffing two documents in their head.

For an Issue worked without a plan, the same three headings go in the MR description instead.

---

## Settling, in one paragraph

In the **same MR** as the work: update the domain's `SPEC.md` to match what changed and cite any
record accepted for it; open an ADR Issue for anything the work turned up that passes the accident
test and is not already recorded; and, **if this Issue was the last of its plan's Issues**, move
the plan directory to `.workflow/plans/archive/<date>-<slug>/` with `Settled: <date>` in its
header. A plan with phases still ahead of it stays where it is.

An Issue with no behaviour change settles in one line in the MR description and touches no files.
That has to stay cheap, or the step gets skipped on the small tasks that are most of the board.

---

## What is checked, and what is not

`wf spec-coverage` runs in about a second, and `/pre-mr-check` runs it whenever anything under
`.workflow/specs/`, `.workflow/decisions/` or `.workflow/plans/` changes — as the project's CI
should. It fails when:

- a spec's `[ADR-NNNN]` link points at no record, at a different record, or at a **superseded**
  one — the behaviour changed and the spec did not follow — or an `ADR-NNNN` appears without a link;
- an accepted record is not cited by the spec its own `spec:` names, unless that is `none`;
- a record's frontmatter is incomplete, its `id` is not its file's number or is taken twice, its
  status is not `accepted` or `superseded`, a supersession points at nothing later, or a record's
  `supersedes:` and the old record's `superseded_by:` do not name each other;
- a record has no row in the index, a row disagrees with its record, or `Supersedes` and
  `Superseded by` do not name each other;
- a directory in `.workflow/plans/` is not named `<DD-MM-YYYY>-<slug>`.

A plan in flight for more than 30 days is printed as debt, never failed on: counted, and meant to
go down.

None of this verifies prose against behaviour. *"A failed order is refunded in full"* is not
greppable. The check proves the links hold and the records agree; whether the spec is still *true*
is a person's job, done in the MR that made it untrue.
