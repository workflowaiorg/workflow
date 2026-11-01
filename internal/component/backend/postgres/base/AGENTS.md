## Database

The backend keeps its data in PostgreSQL, through bun, in one pool per service that every
repository shares. The schema changes only through a new goose migration in `migrations/`, which
`docker compose up -d` applies. The `database` skill says how.
