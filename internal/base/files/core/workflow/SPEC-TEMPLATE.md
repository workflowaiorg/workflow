# `<domain>` — what is true now

Copy this file to `.workflow/specs/<domain>/SPEC.md` when a domain gets its first spec, and delete
the generated line above the title. Delete every section you have nothing to say about; an empty
heading is worse than a missing one.

**This file is edited in place and keeps no history.** It describes the present tense only. Why
it is this way lives in the records it cites; what we are about to change lives in a `PLAN.md`.

**Before writing anything, read the rule in the `domain-specs` skill, *The one rule a spec lives
by*:** if a machine already checks it, this file does not repeat it. No response shapes, no route
tables — whatever CI already checks. Link to them instead.

**Cite a record at the point of the claim**, as a link a reader can follow:
`→ [ADR-NNNN](../../decisions/NNNN-<slug>.md)`. The link text is `[ADR-NNNN]` exactly — that
spelling is what `wf spec-coverage` looks for.

---

## What this domain is

Two or three sentences. What it owns, and where its edges are — the services it talks to and what
it refuses to do on their behalf.

**Code:** `<directory>/`
**Pinned elsewhere:** the contracts CI checks for it, if it has any

## Behaviour

The substance. One `###` per rule, invariant or state machine. Present tense, prose, no
pseudocode. Say what happens, then what cannot happen.

### <A named rule>

What is true. What a reader would otherwise have to derive from three files.
→ [ADR-NNNN](../../decisions/NNNN-<slug>.md)

## State

Any state machine, as a table of transitions — which are allowed and, more usefully, which are
not and why. If the domain writes only one status for the life of a row, say that; it is exactly
the kind of fact someone "fixes" by adding the missing ones.

| From | To | When | Notes |
|---|---|---|---|

## Failure and retry

What happens when the thing fails halfway. What is idempotent and what is not. What is acked
before the work, and what after — and what a duplicate delivery costs.

## Deliberately not built

The most valuable section, and the one most often left empty. Each line stops someone building
something that was already weighed and rejected.

- `<the thing>` — not built because … → [ADR-NNNN](../../decisions/NNNN-<slug>.md)
