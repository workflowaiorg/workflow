---
name: testing
description: "How backend code is tested: the external test package, table-driven cases, controller tests with stand-ins, handler tests through Gin, and gateway tests against an in-process server."
---

# Skill: Testing

## When this skill applies

- Writing or changing any Go test
- Making a stand-in for an interface

## Rules

- **The external test package**: `package orders_test`, never `package orders`. A helper only the
  tests need goes in `export_test.go`.
- Beside the code, as `<file>_test.go`.
- **Table-driven** for the cases of one behaviour: `tests := []struct{ name string; … }` and
  `t.Run(test.name, …)`.
- `github.com/stretchr/testify`: `require` when the test cannot go on, `assert` otherwise.
- **No real network, no `os.Setenv`**: build a `Config` literal, use `t.Setenv` when the
  environment is the thing tested, and `t.Context()` for a context.
- `go test ./...` runs every test; a test that needs a database or another server says so with a
  build tag, so the plain run stays fast.

## Stand-ins

An interface is defined where it is used (the controller defines its `Repository`), so its stand-in
is small. Write it by hand for an interface of a method or two:

```go
type repository struct {
    order *model.Order
    err   error
}

func (r repository) StoreOrder(context.Context, model.NewOrder) (*model.Order, error) {
    return r.order, r.err
}
```

For a larger one, generate it with mockery into a `mocks/` directory beside the interface, and use
its `mocks.NewXxx(t)`, which checks the expected calls when the test ends.

## A controller test

The real controller with a stand-in below it: each business rule, and each error from below
passed on.

## A handler test

The real handler with a stand-in controller, through Gin. `openapitest.Engine(t)` is `gin.New()`
that also checks every response against the service's committed `openapi.yaml`, errors included,
so a body the contract does not allow fails the test:

```go
r := openapitest.Engine(t)
r.POST("/orders", orderhttp.New(controller{order: &model.Order{ID: 7}}).Place)

w := httptest.NewRecorder()
r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/orders", strings.NewReader(`{"customer_id":1,"skus":["a"]}`)))

assert.Equal(t, http.StatusCreated, w.Code)
assert.JSONEq(t, `{"data":{"order":{"id":7}}}`, w.Body.String())
```

A test that calls a handler method directly takes its context from `openapitest.Context(t, w)`
instead of `gin.CreateTestContext`, and is checked the same way. Assert the whole body with
`JSONEq`. A handler test that starts failing means the response changed
shape: find out why before touching the expectation. `api/internal/handler/health/http` has one to
copy.

## An HTTP gateway test

Against `httptest.NewServer`, never the real service: check the method, path and body the gateway
sends, and that each error status becomes the right `APIError` code.

## Definition of done

- [ ] Every test file is `package <pkg>_test`
- [ ] No test reaches the network or calls `os.Setenv`
- [ ] Each behaviour's cases are one table
- [ ] Handler tests serve through `openapitest`, never a bare `gin.New()`
- [ ] `go test ./...` passes
