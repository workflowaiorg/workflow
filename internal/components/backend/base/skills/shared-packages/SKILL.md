---
name: shared-packages
description: "The packages every service shares under pkg/ — logging, errors, httputil and middleware — what each gives, and when a new one belongs there."
---

# Skill: Shared packages

The root `pkg/` holds what every service needs, written once. `wf add backend` wrote the first
four; they are the project's own, to change and extend.

## When this skill applies

- Using `pkg/logging`, `pkg/errors`, `pkg/httputil` or `pkg/middleware`
- Writing code a second service needs, which goes here rather than into a copy

## `pkg/logging`

| | |
|---|---|
| `Init()` | first line of every `main()`: JSON lines on stdout |
| `Debug`, `Info`, `Warn`, `Error(ctx, msg, args...)` | log with the correlation ID `ctx` carries |
| `WithCorrelationID`, `CorrelationID` | put the ID in a context, read it back |

## `pkg/errors`

| | |
|---|---|
| `APIError{Code, Message, Cause}` | the one error that crosses layers; `Cause` is never sent to a client |
| `New(code, message, cause)` | make one |
| `HTTPStatus(code)`, `FromHTTPStatus(status)` | the HTTP status of a code, and the code of a status another service answered with |
| `As`, `Is` | the standard ones, so a file needs only this errors package |

Add a code when a client must tell a new kind of failure apart, and give it its status in both
functions.

## `pkg/httputil`

| | |
|---|---|
| `Success(c, data)`, `Created(c, data)` | answer 200 or 201 with `{"data": …}` |
| `RespondWithError(c, err)` | answer `{"error": {"code", "message"}}` with the code's status; logs the cause |
| `BindError(err)` | the `APIError` for a request `ShouldBind*` rejected |

## `pkg/middleware`

| | |
|---|---|
| `CorrelationID()` | gives each request its `X-Request-ID`, or a new one, in its context and its response |

A gateway that calls another service passes the correlation ID on in the same header, so one
request can be followed through every service's logs.

## A new shared package

Code goes here when a second service needs it, moved rather than copied, with its tests. It depends
only on other `pkg/` packages and libraries, never on a service.

## Definition of done

- [ ] Nothing in a service repeats what a `pkg/` package does
- [ ] A new shared package has its own tests and imports no service
