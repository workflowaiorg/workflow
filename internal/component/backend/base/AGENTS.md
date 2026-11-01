## Backend

A Go backend of services. Each service is a directory at the root (`api/` is the first) and an
entry in `docker-compose.yaml`; the packages every service shares are in `pkg/`.

1. **`logging.Init()` first in every `main()`.** Log through `pkg/logging`, passing the request's
   `ctx`, never through `log`, `slog` or `fmt.Println`.
2. **Dependencies point inward only:** Handler → Controller → Gateway or Repository. No layer
   imports one above it.
3. **Every error that crosses a layer is a `pkg/errors.APIError`.** A gateway or repository never
   lets a raw driver, HTTP or gRPC error out.
4. **Settings are a struct.** Each service reads its environment once, in
   `internal/config`'s `Load()`; nothing else calls `os.Getenv`.
5. **Each transport is a directory of its own:** `handler/<domain>/http/`, `gateway/<domain>/grpc/`.
6. **Tests are in the external package**, `package foo_test`.
7. **Code two services need goes in `pkg/`**, never copied into each.

The skills say how: `architecture` (a service's layout, and a new service), `api-design` (the
layers), `configuration`, `error-handling`, `shared-packages`, `testing` and `go-style`.
