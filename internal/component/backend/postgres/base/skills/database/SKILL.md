---
name: database
description: "How a service keeps data in PostgreSQL: the shared pool, repositories with bun, models, transactions, goose migrations and repository tests."
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
database is still starting. Every repository gets the pool from the `App`; none opens its own.

`Config.PostgresDSN` is built in `Load()` from `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`,
`POSTGRES_PASSWORD`, `POSTGRES_DB` and `POSTGRES_SSLMODE`, each defaulting to the local stack's.

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
- **Any other error** is logged with `logging.Error(ctx, …, "error", err)` and returned as
  `errors.CodeInternal`. No bun or driver error leaves the repository.

`errors` is `pkg/errors`, as everywhere.

```go
func (r *Repository) GetOrder(ctx context.Context, orderID int64) (*model.Order, error) {
    order := new(model.Order)
    err := r.db.NewSelect().Model(order).Where("id = ?", orderID).Scan(ctx)
    switch {
    case errors.Is(err, sql.ErrNoRows):
        return nil, errors.New(errors.CodeNotFound, "No such order", err)
    case err != nil:
        logging.Error(ctx, "getting an order", "error", err, "order_id", orderID)
        return nil, errors.New(errors.CodeInternal, "Could not load the order", err)
    }
    return order, nil
}
```

## Transactions

Writes that must succeed or fail together run in one transaction. `r.db.RunInTx` fits most; use
`BeginTx` with `defer tx.Rollback()` when a step needs what an earlier one returned, such as a new
row's ID.

```go
err := r.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
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
it `depends_on: migrate: condition: service_completed_successfully`, as `api` has.

## Testing a repository

A repository test runs against a real PostgreSQL in a container, with
[testcontainers-go](https://golang.testcontainers.org)'s postgres module, and carries the
`integration` build tag so `go test ./...` stays fast; `go test -tags=integration ./...` runs it.
It applies the migrations with goose's library, `goose.SetDialect("postgres")` then
`goose.Up(db.DB, "<path to migrations/>")` (`db.DB` is the `*sql.DB` under bun), inserts
its own rows, and empties the tables between tests. Those rows never leave the test file.

## Definition of done

- [ ] Repositories take the `App`'s pool; none opens a connection of its own
- [ ] `sql.ErrNoRows` is `CodeNotFound`; every other error is a logged `CodeInternal`
- [ ] Writes that belong together run in one transaction
- [ ] Models embed `bun.BaseModel` with a table and alias; nullable columns are pointers
- [ ] A schema change is a new migration; no applied migration was edited; no data in migrations
- [ ] A new query has a repository test
