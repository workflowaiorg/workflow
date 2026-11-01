---
name: architecture
description: "How a backend service is laid out and started: its directories, its main(), the App container, graceful shutdown, where shared code lives, how to run the stack locally, and how to add a new service."
---

# Skill: Service architecture

Every service is built the same way, so anyone who knows one knows them all.

## When this skill applies

- Adding a new service
- Changing a `cmd/` entrypoint, `internal/app/`, startup or shutdown
- Deciding whether code belongs to one service or to `pkg/`
- Starting, stopping or checking the app locally

## A service's layout

```text
<service>/
├── cmd/http/main.go          # the entrypoint; cmd/grpc/ for a gRPC service
└── internal/
    ├── app/app.go            # App: settings and connections opened once, shared by the routes
    ├── config/config.go      # Config and Load(): see the configuration skill
    ├── controller/<domain>/  # business logic; defines the interfaces it needs
    ├── gateway/<domain>/<transport>/     # calls to other services: http/, grpc/
    ├── repository/<domain>/<driver>/     # the database: postgres/
    ├── handler/<domain>/<transport>/     # requests in: http/, grpc/
    ├── model/                # the domain's structs, shared by the layers
    └── route/<domain>.go     # wiring: gateway/repository → controller → handler → routes
```

A transport is always a directory of its own: HTTP and gRPC code never share a package. The api-design
skill says what each layer may do.

## main()

The order never changes:

1. `logging.Init()`.
2. `cfg := config.Load()`, then check every required setting and `os.Exit(1)` with a message naming
   the variable when one is missing.
3. `app.New(cfg)`: open what is opened once, such as a database pool;
   `main` closes it once the server has stopped.
4. `gin.New()` (never `gin.Default()`), with `gin.Recovery()` and `middleware.CorrelationID()`, and
   `SetTrustedProxies` set explicitly.
5. Register each domain's routes; a registration that returns an error is logged and exits 1.
6. Serve until SIGINT or SIGTERM, then shut down with a timeout, so requests in flight finish.

`api/cmd/http/main.go` is the reference: copy it.

**A service no client calls directly** serves only the routes other services call, under
`/internal/<service>/`, so one rule at the edge can keep them all private. Authentication happens
once, in the service that faces clients; services behind it trust its calls and do not check again.

## The App container

`internal/app.App` holds the settings and what is opened once. Routes receive `*app.App`; it is a
struct, never an interface. A helper every handler needs, such as the signed-in user, is a method
on it.

## Where code goes

| Where | For |
|---|---|
| `pkg/` at the root | code more than one service uses: logging, errors, HTTP helpers, middleware |
| `<service>/pkg/` | code shared inside one service only |

Move code to the root `pkg/` only when a second service needs it, and then move it rather than
copy it.

## Running it

The local stack is `docker-compose.yaml` at the project's root; run compose there.

- Start: `docker compose up -d`. The first start compiles for a minute.
- Check: `curl -sf --retry 30 --retry-all-errors http://localhost:8000/health` answers once `api`
  has compiled; each service has its own host port.
- Logs: `docker compose logs <service>`; add `-f` only in a terminal a person watches, since it
  never returns.
- Stop: `docker compose down`.

## A new service

1. Copy `api/` to `<service>/` and replace `api` in its import paths.
2. Keep `internal/config`, `internal/app`, the health route and `cmd/http/main.go`; remove the
   domains it does not have.
3. Add it to `docker-compose.yaml`, as `api` is, with a host port of its own.
4. `go vet ./... && go test ./...`, then `docker compose up -d`, and check that its `/health`
   answers on its port, as *Running it* says.

## Definition of done

- [ ] The layout matches the tree above, with each transport in a directory of its own
- [ ] `main()` follows the six steps, failing fast on a missing setting
- [ ] Shared state hangs off `App`, not package variables
- [ ] Shutdown is graceful
- [ ] Code shared by services is in the root `pkg/`, and only there
