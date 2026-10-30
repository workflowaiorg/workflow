# Decision records

Why the system is the way it is, and what we turned down. One file per decision. A record is never
edited after it merges — a decision that changes gets a new record that supersedes the old one.

Most decisions never earn a record. The test for the ones that do, and how a record is accepted,
are in the `domain-specs` skill, *When a decision earns a record*. The short version: a record is
agreed in an `[ADR]` Issue **before** the code that relies on it.

To add one: copy [`../DECISION-TEMPLATE.md`](../DECISION-TEMPLATE.md) to `NNNN-<slug>.md`, take the
next free number, and add its row below in the same commit. `wf spec-coverage` checks that every
record has its row and that each row says what its record says.

## The index

One row per record, newest last. A superseded record keeps its row and its file forever;
`Supersedes` and `Superseded by` point at each other.

| ID | Title | Domain | Status | Issue | Supersedes | Superseded by |
|---|---|---|---|---|---|---|

## Why every record sits in this one directory

The records that matter most belong to no single domain. Filing records under a domain would force
a choice of folder for exactly those. So they all live here, and the `domain:` field in each file's
header is what you filter by.

The number is just a short name: something you can say out loud in review — "see 0007" — and link
to from a spec.

Two branches can both take `0031`. Whoever merges second renames their file and fixes its row.
