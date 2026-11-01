---
name: error-handling
description: "How errors travel through a service and how it logs: APIError across every layer, what each layer does with an error, panic recovery, and logging with the request's context."
---

# Skill: Error handling and logging

## When this skill applies

- Returning an error from a repository, gateway, controller or handler
- Turning a driver, HTTP or gRPC error into one of ours
- Adding a log line, or panic recovery for a new server

## How an error travels

| Layer | Does |
|---|---|
| Repository, gateway | Turns every error from outside into a `pkg/errors.APIError` with the code that fits, keeping the original as its cause. Nothing raw leaves |
| Controller | Passes an `APIError` on unchanged, or returns a new one when a domain rule fails |
| HTTP handler | `httputil.RespondWithError(c, err)`: an `APIError` keeps its code and message; anything else is a 500 that says nothing of its cause |
| gRPC handler | Turns the `APIError` back into a gRPC status with the mapping in `pkg/`, never one of its own |

```go
order, err := r.db.FindOrder(ctx, id)
if errors.Is(err, sql.ErrNoRows) {
    return nil, errors.New(errors.CodeNotFound, "No such order", err)
}
if err != nil {
    return nil, errors.New(errors.CodeInternal, "Could not load the order", err)
}
```

`errors` here is `pkg/errors`. It has `Is` and `As` too, so a file needs no other errors package.

The message is for the client: it says what went wrong in their terms. The cause is for the logs.

## Panics

`gin.Recovery()` in `main()` recovers every HTTP handler; handlers add no recovery of their own.

**The first gRPC service** adds two things to `pkg/` (as `pkg/grpcutil`): the mapping between an
`APIError`'s code and a gRPC status, both ways, and a recovery interceptor for its server. Every
gRPC service after it uses those, and writes neither again.

## Logging

Through `pkg/logging` only, always with the request's context, so the correlation ID travels:

```go
logging.Info(ctx, "order placed", "order_id", order.ID)
logging.Error(ctx, "payment failed", "order_id", order.ID, "error", err)
```

- In an HTTP handler, `ctx` is `c.Request.Context()`; in a gRPC handler, the method's `ctx`.
- Never `log`, `slog` or `fmt.Println` directly.
- An error is logged once, where it is handled: `RespondWithError` logs it, so the layers below
  return it without logging.

## Definition of done

- [ ] Every error crossing a layer is an `APIError`, with the original as its cause
- [ ] Handlers answer errors through the shared helpers
- [ ] Every log call gets the request's `ctx`
- [ ] Each error is logged once
