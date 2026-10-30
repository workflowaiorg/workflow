---
id: NNNN
title: <the decision in one line, present tense>
date: <YYYY-MM-DD>
status: accepted
domain: <the domain it belongs to, or none>
spec: <.workflow/specs/<domain>/SPEC.md, or none>
supersedes: <the NNNN of the record this one replaces, or —>
superseded_by: —
issue: <the number of the ADR Issue this was accepted in>
---

<!--
Copy this file to .workflow/decisions/NNNN-<slug>.md, take the next free NNNN, and add its row
to .workflow/decisions/README.md in the same commit.

Do not write this file until the ADR Issue has been accepted in its thread. The Issue is the
proposed state — see the domain-specs skill, "How a record is accepted". There is no `proposed`
status here, and `accepted` is not yours to assign on your own judgement.

Records are never edited afterwards. A decision that changes gets a new record, a new Issue, and
`supersedes:` naming the old one, which gets `superseded_by:` filled in. Delete these comments
and the generated line above them.
-->

## Decision

What we are committing to, in the present tense, in a few sentences. Not the debate — the
outcome. Someone reading only this section should be able to tell whether code in front of them
obeys it.

## Alternatives considered

Each option that was genuinely on the table, and why it was turned down. This is the half that
stops the decision being re-litigated every six months, so an alternative dismissed in one clause
is worth more than one omitted.

- **<the option>** — rejected because …
- **<the option>** — rejected because …

## Consequences

What we accept by choosing this, **including what gets worse**. A record with only upsides was
not a decision, it was a preference, and it will not survive contact with the person who hits the
downside without warning.

## Undoing this by accident

One or two sentences naming the shape of the mistake: what a future developer or agent would see
that looks like a bug, and what they would break by "fixing" it. This is the section that earns
the record its place — if you cannot write it, the decision probably did not need a file.
