---
name: api-design
description: "The layers of a service and what each may do: handlers, controllers, gateways, repositories, routes, models and response bodies, with dependencies pointing inward only; and the OpenAPI contract every public route is in."
---

# Skill: API design

Dependencies point inward only: **Handler → Controller → Gateway or Repository**. No layer imports
one above it, and each layer defines, as an interface, what it needs of the layer below; the
concrete types meet only in `route/`.

## When this skill applies

- Writing a handler, controller, gateway or repository
- Wiring a domain in `internal/route/`, or adding a route
- Writing a response body, a model, or a service's own middleware
- Changing anything a public route reads or answers with: its operation, `openapi.yaml`

## Handler: `internal/handler/<domain>/<transport>/`

- Defines a `Controller` interface with the methods it calls, and receives the controller (and
  `*app.App` when it needs it) in `New`.
- Reads and validates the request, calls the controller, and answers with `pkg/httputil`.
- Holds no business logic and calls no gateway.
- Binds a request body, or a query, to a named, unexported struct declared just above the method,
  so the route's operation can name it. A query is bound with `httputil.BindQuery`, never read by
  name; a path ID with `httputil.ParamID`. A type is read or answered with, never both: a request
  holds types of its own, as the document gives each direction its own required fields.

```go
type Controller interface {
    Place(ctx context.Context, customerID int64, skus []string) (*model.Order, error)
}

// placeRequest is the body of POST /orders.
type placeRequest struct {
    CustomerID int64    `json:"customer_id" binding:"required"`
    SKUs       []string `json:"skus" binding:"required,min=1"`
}

func (h *Handler) Place(c *gin.Context) {
    var req placeRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        httputil.RespondWithError(c, httputil.BindError(err))
        return
    }
    order, err := h.ctrl.Place(c.Request.Context(), req.CustomerID, req.SKUs)
    if err != nil {
        httputil.RespondWithError(c, err)
        return
    }
    httputil.Created(c, orderResponse{Order: order})
}
```

## Response bodies

Every response body is a named, unexported struct in the handler package's `response.go`, never a
`gin.H`, so the package's whole public shape is in one file.

- A collection is named by its plural noun; it has `has_more` only when the endpoint takes an
  offset.
- A key that is sometimes absent is a pointer with `omitempty`: absent, not `null`. A key clients
  rely on is never `omitempty`: removing an optional key breaks no check, so it was never promised.
- Keys are `snake_case`. An ID is a number. A time is an RFC 3339 string, which a `time.Time`
  is. Money is an integer in minor units, such as cents, with its currency code beside it:
  `"price": 1250, "currency": "EUR"`.

## Controller: `internal/controller/<domain>/`

- Defines a `Gateway` or `Repository` interface with what it needs, and receives it in `New`.
- Holds the business rules; returns models or `APIError`s.
- Knows nothing of HTTP, gRPC or Gin.

```go
type Repository interface {
    StoreOrder(ctx context.Context, order model.NewOrder) (*model.Order, error)
}

type Controller struct{ repo Repository }

func New(repo Repository) *Controller { return &Controller{repo: repo} }
```

## Gateway: `internal/gateway/<domain>/<transport>/`

A call to another service. It implements the controller's interface, turns every failure into an
`APIError` (error-handling skill), and returns models, never the other service's types.

- **HTTP**: `New(baseURL string) *Gateway`. It builds each URL from the base URL; `http.DefaultClient`
  is not used, as it has no timeout.
- **gRPC**: `New(addr string, opts ...grpc.DialOption) (*Gateway, error)`, taking dial options so a
  test can pass an in-memory listener, and with `Close() error`.

## Repository: `internal/repository/<domain>/<driver>/`

The database, behind the controller's interface. It returns models and `APIError`s, never driver
types or driver errors. `New` takes the connection it reads from; it does not open one.

## Route: `internal/route/<domain>.go`

One file per domain. It builds gateways and repositories, then the controller, then the handler,
then registers the routes. When building something fails it returns the error, closing whatever it
had already opened, and `main()` exits. `ctx` ends when the service is told to stop; work the route
starts in the background stops with it.

```go
func RegisterOrderRoutes(ctx context.Context, application *app.App, r gin.IRouter) error {
    payments, err := paymentsgrpc.New(application.Config.Payments.Addr)
    if err != nil {
        return fmt.Errorf("payments gateway: %w", err)
    }
    ctrl := orderctrl.New(application.Orders, payments)
    handler := orderhttp.New(ctrl)

    orders := r.Group("/orders")
    orders.POST("", handler.Place)
    orders.GET("/:id", handler.Show)
    return nil
}
```

## The OpenAPI contract: `<service>/openapi.yaml`

A service's `openapi.yaml` is the contract its clients are built against. It is generated from the
Go types and committed; nobody edits it by hand.

**Every public route has an operation** in its handler package's `openapi.go`, beside the types it
names:

```go
func Operations() []openapi.Operation {
    return []openapi.Operation{
        {Method: "GET", Path: "/orders", Query: listQuery{}, Response: ordersResponse{}},
        {Method: "POST", Path: "/orders", Request: placeRequest{}, Status: http.StatusCreated, Response: orderResponse{}},
        {Method: "GET", Path: "/orders/:id", Response: orderResponse{}},
    }
}
```

`Path` is Gin's, group prefix included. `Response` is what `httputil.Success` puts under `data`:
the document adds the envelope itself, and gives every operation the error envelope for any other
status. `Status` is set only when success is not 200; a redirect has no `Response`. A new handler
package adds its `Operations()` to `route.Operations()`, and its routes to `route.Register`. A
paged list's query is `httputil.PageQuery`, or a struct that embeds it.

**Then regenerate, and commit the file**, after changing a route, an operation, a type a route
reads or answers with (a request, a response, a model either embeds), `pkg/httputil` or
`pkg/errors`:

```bash
go run ./api/cmd/openapi
```

**What fails, and where:**

| A change that… | Fails in |
|---|---|
| registers a route with no operation, or leaves an operation whose route is gone | `TestRegisterMatchesTheOperations`, in `internal/route/` |
| changes a type or a route and leaves `openapi.yaml` behind, or edits it by hand | `TestTheOpenAPIDocumentIsUpToDate`, in `internal/route/` |
| answers with a body `openapi.yaml` does not allow | the handler's own tests, through `openapitest`: the `testing` skill |
| removes a required field or a route, or changes anything else a client relies on | the CI's breaking-change job, unless the MR carries `api-passthrough` |

`go test ./...` runs the first three. The last runs only in CI, where the MR's labels are known.

**Breaking a client on purpose**, removing or renaming a required field or a route, takes the
`api-passthrough` label on the MR: the job prints every break and passes, and the reviewer accepts
them by approving. Use it only for what no client outside the repository relies on yet.

**Retiring a required field or a route without the label** takes two MRs. The first deprecates it
with a sunset date no earlier than the day it merges: `Sunset: "2027-03-01"` on the operation, or
`deprecated:"true" sunset:"2027-03-01"` on the field. Once the date has passed, the second removes
it, and the breaking-change job lets the removal through.

## Model: `internal/model/`

- The domain's structs, used by every layer, with `json` tags.
- One file per domain entity (`order.go` holds `Order`, `NewOrder` and `Item`), never a `model.go`
  catch-all.
- A write that needs only some fields takes an input struct of its own, such as `NewOrder`, rather
  than a half-filled model or a list of arguments.

## Middleware

Middleware every service uses is in `pkg/middleware`; a service's own, such as reading its
signed-in user, is in its `internal/middleware`.

## Definition of done

- [ ] Each layer imports only the layer below, through an interface it defines
- [ ] Handlers hold no business logic; controllers know no transport
- [ ] Every response body is a named struct in `response.go`
- [ ] Gateways and repositories return models and `APIError`s only
- [ ] Each domain is wired in one `route/` file, closing what it opened when it fails
- [ ] Every public route has an operation, and `openapi.yaml` was regenerated and committed
- [ ] A break a client would see has a sunset that has passed, or the MR carries `api-passthrough`
