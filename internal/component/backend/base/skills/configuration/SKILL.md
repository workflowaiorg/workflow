---
name: configuration
description: "How a service reads its settings: one Config struct per service, filled once by Load() from the environment, checked at start, and never read anywhere else."
---

# Skill: Configuration

## When this skill applies

- Adding or reading an environment variable
- Writing or changing a `Config` or its `Load()`
- Checking a required setting at start

## Rules

- **Every service has `internal/config`** with a `Config` struct and a `Load()` function, even a
  service with one setting. Tests build a `Config` directly, and `Load()` can later read files or a
  secret store without any caller changing.
- **`Load()` is the only place that reads the environment.** A constructor receives the `Config`,
  or the fields it needs; nothing else calls `os.Getenv`.
- A setting with a sensible default has it in `Load()`; a required one is checked in `main()`.
- Group the settings of one concern in a nested struct: `cfg.Orders.BaseURL`.
- The standard library only: no configuration framework.

```go
type Config struct {
    Port   string
    Orders struct {
        BaseURL string
    }
}

func Load() *Config {
    cfg := &Config{Port: getenv("PORT", "8000")}
    cfg.Orders.BaseURL = getenv("ORDERS_BASE_URL", "http://orders:8000")
    return cfg
}
```

**The address of another service** defaults to its name in `docker-compose.yaml`, so the local
stack works with no setting at all.

**A database** is reached through one DSN field, built in `Load()` from its parts, so a test passes
its own DSN in a `Config` literal.

## Checking at start

Right after `config.Load()` in `main()`:

```go
if cfg.Payments.APIKey == "" {
    logging.Error(ctx, "PAYMENTS_API_KEY is not set")
    os.Exit(1)
}
```

Name the variable, not the field: it is what the developer sets.

## Where the values come from

The values a service needs locally go in its compose entry (`environment:`), or in an env file it
loads (`env_file:`). An env file with secrets is gitignored, and a committed `.example` beside it
lists every key with a usable default or an empty placeholder. A new variable is two edits in one
change: the field in `Load()`, and the key in the example.

## Definition of done

- [ ] Every setting is a field of `Config`, read once in `Load()`
- [ ] No `os.Getenv` outside `Load()`
- [ ] Each required setting is checked in `main()`, naming the variable
- [ ] Each new variable is in the compose entry or the env file's example, in the same change
