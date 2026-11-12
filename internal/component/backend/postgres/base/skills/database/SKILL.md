---
name: database
description: "How a service keeps data in PostgreSQL: the shared pool, repositories with bun, models, transactions with postgres.RunInTx, goose migrations and repository tests with postgrestest."
---

# Skill: Database

The data is in PostgreSQL, reached through [bun](https://bun.uptrace.dev). The schema changes only
through [goose](https://github.com/pressly/goose) migrations.

## When this skill applies

- Writing or changing a repository: a query, an insert, a transaction
- Adding or changing a model's bun tags or relations
- Changing the schema: a table, a column, an index
- Giving a new service the database
- Testing a repository against a real database

## The pool

Each service opens one pool, in `app.New`, with `pkg/postgres.Open(cfg.PostgresDSN)`, and closes
it after the server stops. It connects on the first query, so the service starts while the
database is still starting. Every repository gets the pool from the `App` and none opens its own,
so two repositories on one database share one pool.

`Config.PostgresDSN` is built in `Load()` from `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`,
`POSTGRES_PASSWORD`, `POSTGRES_DB` and `POSTGRES_SSLMODE`. The password has no default and sslmode
defaults to `require`, so a deploy never connects with the local stack's password or without TLS
by accident; `docker-compose.yaml` gives the service the local values. `require` encrypts without
checking the server's certificate; a deploy that must check it sets `verify-full`.

## Repository: `internal/repository/<domain>/postgres/`

`postgres.go` holds only the `Repository` and `New`; each other file holds the methods of one
concern, such as `order.go` and `line.go`.

```go
type Repository struct {
    db *bun.DB
}

func New(db *bun.DB) *Repository {
    return &Repository{db: db}
}
```

The route builds it from the `App`: `orderpg.New(application.DB)`.

- **Not found** (`sql.ErrNoRows`) is `errors.CodeNotFound`, as `error-handling` says.
- **Any other error** is returned as `errors.CodeInternal`, with it as the cause. No bun or driver
  error leaves the repository, and the repository does not log it: `error-handling` says where it
  is logged.

`errors` is `pkg/errors`, as everywhere.

```go
func (r *Repository) GetOrder(ctx context.Context, orderID int64) (*model.Order, error) {
    order := new(model.Order)
    err := r.db.NewSelect().Model(order).Where("id = ?", orderID).Scan(ctx)
    switch {
    case errors.Is(err, sql.ErrNoRows):
        return nil, errors.New(errors.CodeNotFound, "No such order", err)
    case err != nil:
        return nil, errors.New(errors.CodeInternal, "Could not load the order", err)
    }
    return order, nil
}
```

## Transactions

Writes that must succeed or fail together run in one transaction, through
`postgres.RunInTx(ctx, r.db, fn)` from `pkg/postgres`, never bun's own `RunInTx` or `BeginTx`. It
rolls back when `fn` returns an error and commits otherwise. An `APIError` from `fn` comes back as
it is; any other error, a failed begin or commit among them, comes back as `CodeInternal`. So
`fn` returns bun's errors unwrapped, and wraps only what has a code of its own.

```go
err := postgres.RunInTx(ctx, r.db, func(ctx context.Context, tx bun.Tx) error {
    if _, err := tx.NewInsert().Model(order).Exec(ctx); err != nil {
        return err
    }
    for i := range lines {
        lines[i].OrderID = order.ID
    }
    _, err := tx.NewInsert().Model(&lines).Exec(ctx)
    return err
})
```

- **A value out of a transaction** is a variable declared before it, which `fn` sets. An insert
  fills in the model's `id`, as `order.ID` above.
- **Rolling back without failing**, when `fn` finds there is nothing to do, is
  `return postgres.ErrRollback`: `RunInTx` rolls back and returns nil.

## Models

A model embeds `bun.BaseModel` with its table and alias, and has `json` tags too when it is also
an answer.

```go
type Order struct {
    bun.BaseModel `bun:"table:orders,alias:o"`

    ID        int64      `bun:"id,pk,autoincrement"                  json:"id"`
    Customer  string     `bun:"customer"                             json:"customer"`
    PaidAt    *time.Time `bun:"paid_at"                              json:"paid_at"`
    CreatedAt time.Time  `bun:"created_at,default:current_timestamp" json:"created_at"`
}
```

- A primary key is `bun:"id,pk,autoincrement"`.
- A column the database fills in has `default:`, so bun leaves it out of an insert.
- A nullable column is a pointer: `NULL` is `nil`.
- A relation is a pointer field, `bun:"rel:belongs-to,join:customer_id=id"`, loaded only when a
  query asks for it with `.Relation("Customer")`.

## Migrations: `migrations/`

Every schema change is a new SQL file in `migrations/` at the root, shared by every service:
`<YYYYMMDDHHMMSS>_<what_it_does>.sql`, with `-- +goose Up` and `-- +goose Down` sections.

- A migration that has been applied anywhere but your machine is never edited: the change is a new
  file.
- A migration holds schema, never data to click around with: migrations run in production too.
- `docker compose up -d` applies the new ones: the `migrate` service runs before every service that
  keeps data. After adding one to a running stack, `docker compose up -d migrate` applies it, and
  `docker compose logs migrate` shows what it did.

## A new service with the database

Copied from `api`, it already reads `POSTGRES_*` and opens the pool. In `docker-compose.yaml`, give
it the `environment:` and `depends_on: migrate: condition: service_completed_successfully` that
`api` has.

## Testing a repository

A repository test runs against a real PostgreSQL, and carries the `integration` build tag so
`go test ./...` stays fast and needs no Docker; `go test -tags=integration ./...` runs it.

`postgrestest.Start(t)` from `pkg/postgrestest` starts the database in a container of its own,
with [testcontainers-go](https://golang.testcontainers.org), applies every migration in
`migrations/`, and stops it when the test ends. It returns the `DSN`, which the test opens the
repository's pool with, and `DB`, a `*sql.DB` of the test's own for the rows it inserts and reads
back. A container takes seconds to start, so the cases of one repository share one: they are
subtests of the test that started it, and each inserts the rows it needs. Those rows never leave
the test file.

- **A row takes the ID the database gives it**, read back with `RETURNING id`: an ID written by
  hand leaves the sequence behind, and the repository's next insert collides with it.
- **A case checks only the rows it inserted**, by their IDs or a value of its own, since the rows
  of the cases before it are still there. A list or count query gets a test of its own, with its
  own `Start`.

```go
//go:build integration

func TestOrders(t *testing.T) {
    database := postgrestest.Start(t)
    db := postgres.Open(database.DSN)
    t.Cleanup(func() { _ = db.Close() })
    repository := orderpg.New(db)

    t.Run("an order is found by its ID", func(t *testing.T) {
        ctx := t.Context()
        var orderID int64
        err := database.DB.QueryRowContext(ctx, "INSERT INTO orders (customer) VALUES ('Ann') RETURNING id").Scan(&orderID)
        require.NoError(t, err)

        order, err := repository.GetOrder(ctx, orderID)
        require.NoError(t, err)
        assert.Equal(t, "Ann", order.Customer)
    })
}
```

## Definition of done

- [ ] Repositories take the `App`'s pool; none opens a connection of its own
- [ ] `sql.ErrNoRows` is `CodeNotFound`; every other error is `CodeInternal`, not logged here
- [ ] Writes that belong together run in one `postgres.RunInTx`
- [ ] Models embed `bun.BaseModel` with a table and alias; nullable columns are pointers
- [ ] A schema change is a new migration; no applied migration was edited; no data in migrations
- [ ] A new query has a repository test
