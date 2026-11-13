# Decision records

Why the system is the way it is, and what we turned down. One file per decision. A record is never
edited after it merges — a decision that changes, wholly or in part, gets a new record that
supersedes the old one, and the old one is deleted.

Most decisions never earn a record. The test for the ones that do, and how a record is accepted,
are in the `domain-specs` skill, *When a decision earns a record*. The short version: a record is
agreed in an `[ADR]` Issue **before** the code that relies on it.

To add one: copy [`../DECISION-TEMPLATE.md`](../DECISION-TEMPLATE.md) to `NNNN-<slug>.md`, take the
number one above the highest ever used, in the index or in git history (numbers are never
reused), and add its row below in the same commit. `wf spec-coverage` checks that every
record has its row and that each row says what its record says.

## The index

One row per record, newest last. A record that was superseded, or whose feature was removed from
the code, is deleted with its row, and numbers are never reused, so a gap is a record replaced or
removed. `Supersedes` names the number it replaced; git keeps the old text:

```sh
git log -p -- '.workflow/decisions/NNNN-*'   # the text of record NNNN
git log --format= --name-only -- .workflow/decisions/ | sed -n 's|.*/\([0-9]\{4\}\)-.*|\1|p' | sort -u | tail -1   # the highest number ever used
```

| ID | Title | Domain | Status | Issue | Supersedes |
|---|---|---|---|---|---|

## Why every record sits in this one directory

The records that matter most belong to no single domain. Filing records under a domain would force
a choice of folder for exactly those. So they all live here, and the `domain:` field in each file's
header is what you filter by.

The number is just a short name: something you can say out loud in review — "see 0007" — and link
to from a spec.

Two branches can both take `0031`. Whoever merges second renames their file and fixes its row.
