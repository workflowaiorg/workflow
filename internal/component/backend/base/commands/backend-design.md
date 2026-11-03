---
description: "Talks through what the backend should do, proposes its domains and services, and once the developer agrees, creates the services, writes each domain's spec and describes the system in the project's AGENTS.md."
argument-hint: <what the system should do, in a few sentences>
---

Design the backend with the developer, then lay it out. `$ARGUMENTS` is what they want built; if
it is empty, ask for it in a few sentences and wait.

**Nothing is written before the developer agrees with the design.** Until then this is a
conversation.

## 1. Understand

Read `AGENTS.md`, the `architecture` skill, and what already exists: the services at the root, the
`agents/` directory, `.workflow/specs/`, and the options in `.wf.yml`. A design adds to what is
there; it never quietly replaces it.

Then ask what the description leaves open, three or four questions at a time, never a list of
twenty. Ask only what changes the design:

- who uses it, and what they do with it, step by step;
- what it keeps, and what must never be lost or be wrong, such as money;
- what takes too long for a request and runs in the background;
- what asks a language model, and what that call needs from the rest of the system;
- what it calls outside the project: payments, email, storage.

Stop asking when the answers stop changing the design.

## 2. Propose

A **domain** is one part of the business with its own rules and its own data: orders, billing,
accounts. A **service** is a directory at the root and a process of its own; it holds one domain or
several, and a domain belongs to one service only.

- **Start with as few services as will do.** Each one costs a process, a port and calls between
  them. A domain gets a service of its own only for a reason the developer can name: it is
  scaled, released or trusted differently, or it is a store another service must not reach into.
- **`api` is the one service clients call.** Services behind it serve only `/internal/<service>/`,
  as the `architecture` skill says.
- **Background work** goes on the queue, when the project has the `queue` option.
- **Language-model work** is an agent under `agents/`, when the project has the `llm-agents`
  option: payload in, payload out, so the service that calls it loads and saves its data.
- An option the design needs and the project lacks is said plainly: options are chosen when the
  backend is added, and wf cannot turn one on later, so the design does without it.

Show the design in one reply:

1. a table of services: the service, its domains, what it does in one line, who calls it, and its
   host port, the next one free in `docker-compose.yaml`;
2. a table of domains: the domain, what it owns, and what it refuses to do for others;
3. the calls between services, one line each;
4. what is deliberately not built, and why.

Then ask whether to lay it out. Change the design as often as the developer asks; it is theirs.

## 3. Lay it out

Once they agree, follow the `workflow` skill: the Issue first, its request in the developer's
words and the agreed tables in its description, then its worktree. Then:

- **Each new service**, as the `architecture` skill's *A new service* says: a copy of `api` with
  `/health`, its settings and its own port in `docker-compose.yaml`. Its domains' code is not
  written yet: each feature that fills them is an Issue of its own.
- **Each new agent**, as the `llm-agents` skill says: a copy of `agents/summary`, with its card
  saying what the design gave it to do.
- **Each domain's first spec**, `.workflow/specs/<domain>/SPEC.md`, from
  `.workflow/SPEC-TEMPLATE.md`, as the `domain-specs` skill says of a domain with no code yet: only
  *What this domain is*, with its service as its code, and *Deliberately not built*. A rule nobody
  decided is not written down.
- **The system in `.agents/project/AGENTS.md`**: a `## System` section with the services table,
  without its ports, which `docker-compose.yaml` keeps, and the calls between them, so every later
  task starts from the same picture. Then `wf sync`.

Run `gofmt -l .`, `go vet ./...` and `go test ./...`, then start the stack and check that each new
service's `/health` and each new agent's card answer, as the `architecture` skill's *Running it*
says. Then offer `/pre-mr-check`. Commit nothing until the developer has read the work and said go.
