---
name: api-design
description: "The layers of a service and what each may do: handlers, controllers, gateways, repositories, routes, models and response bodies, with dependencies pointing inward only."
---

# Skill: API design

Dependencies point inward only: **Handler → Controller → Gateway or Repository**. No layer imports
one above it, and each layer defines, as an interface, what it needs of the layer below; the
concrete types meet only in `route/`.

## When this skill applies

- Writing a handler, controller, gateway or repository
- Wiring a domain in `internal/route/`, or adding a route
- Writing a response body, a model, or a service's own middleware

## Handler: `internal/handler/<domain>/<transport>/`

- Defines a `Controller` interface with the methods it calls, and receives the controller (and
  `*app.App` when it needs it) in `New`.
- Reads and validates the request, calls the controller, and answers with `pkg/httputil`.
- Holds no business logic and calls no gateway.
- Binds a request body to a struct declared in the method, unless several methods share it.

```go
type Controller interface {
    Place(ctx context.Context, customerID int64, items []model.Item) (*model.Order, error)
}

func (h *Handler) Place(c *gin.Context) {
    var req struct {
        CustomerID int64        `json:"customer_id" binding:"required"`
        Items      []model.Item `json:"items" binding:"required,min=1"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        httputil.RespondWithError(c, httputil.BindError(err))
        return
    }
    order, err := h.ctrl.Place(c.Request.Context(), req.CustomerID, req.Items)
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
- A key that is sometimes absent is a pointer with `omitempty`: absent, not `null`.

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
