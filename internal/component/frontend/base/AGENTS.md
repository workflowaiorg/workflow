## Frontend

A Vue app in `web/`, written in TypeScript and typed against the backend's API contract,
`api/openapi.yaml`.

1. **Every HTTP call goes through `web/src/services/`.** A component or a store calls a service's
   function, never `fetch`; `npm run lint` fails on a network call anywhere else.
2. **API types are generated, never written.** After `api/openapi.yaml` changes, run
   `npm run api:types` in `web/` and commit `src/services/api.gen.d.ts` with it; `npm run lint`
   fails while it is stale.
3. **Text is rendered with `{{ }}`, never as HTML.** `npm run lint` fails on `v-html` and the
   other raw-HTML sinks.
4. **A screen that calls a new endpoint merges only once that endpoint is live in production.**

The `frontend` skill says how.
