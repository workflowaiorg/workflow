---
name: frontend
description: "How the web app in web/ is built: its stack and layout, components, readable TypeScript, the services every HTTP call goes through, the API types generated from the backend's contract, the order a screen and its endpoint deploy in, and when a screen is done."
---

# Skill: Frontend

## When this skill applies

- Working on any file under `web/`: a view, a component, a composable, a service, the router
- Calling an endpoint from the app, or changing one the app calls

## Stack

| Concern | Technology |
|---|---|
| Language | TypeScript, `strict`, checked by `npm run typecheck` (`vue-tsc`) |
| Framework | Vue 3, Composition API, `<script setup lang="ts">` |
| Routing | Vue Router |
| HTTP | `fetch`, wrapped once in `src/services/api.ts` |
| Build | Vite |

A library the app needs beyond these (a store, a CSS framework, i18n) is the project's choice;
when it is made, write it into this table in the project's part of this skill.

## Layout

```text
web/
├── compose.yaml        # the dev server, included by docker-compose.yaml
├── scripts/lint.mjs    # the checks npm run lint runs
└── src/
    ├── main.ts         # creates the app, installs the router
    ├── App.vue
    ├── components/     # shared components at the root, a domain's in its folder
    ├── composables/    # useXxx.ts
    ├── router/
    ├── services/       # api.ts, api.gen.d.ts, and one <domain>Service.ts per API domain
    └── views/          # route components, a domain's in its folder
```

**A file lives with its domain.** A component that belongs to one domain goes in that domain's
folder, even when a screen of another domain shows it. The root of `components/` is for what no
single domain owns. A component with one caller is folded into it unless it is a unit of its own.
Check this whenever a component gains or loses a caller.

## Components

- Every component is `<script setup lang="ts">`, with the Composition API.
- Props with `defineProps`, events with `defineEmits`, named (`emit('close')`) rather than
  callback props. A value the parent both reads and writes is `defineModel()`.
- Logic two components need goes in `src/composables/useXxx.ts`, never copied into each.
- **Text is rendered with `{{ }}`, never as HTML.** No `v-html`, `innerHTML` or other raw-HTML
  sink: text that users or a model wrote is not safe as markup, and `npm run lint` fails on them.
  When markdown is truly needed, sanitise it where it is rendered and make check 2 in
  `scripts/lint.mjs` require the sanitiser there.

## TypeScript is written to be read

A reader who knows JavaScript should follow a file without studying its types.

- Annotate what a caller sees (parameters, props, emits, an exported function's result) and let
  the checker infer the rest: locals, `ref(0)`, `computed`.
- A type that means something gets a name (`type ItemQuery = Query<'/items'>`); a long type
  expression is never written inline.
- Plain shapes only: `interface`, object types, unions of literals, `Schema<>` and `Query<>`.
  Conditional types, `infer` and mapped types live in `services/api.ts` alone. A generic needs two
  real callers.
- `as` only where the checker cannot see, never to make an error go away. An `any` or a
  `@ts-ignore` carries a comment saying why.

## Services and the API types

- **Every HTTP call goes through `src/services/<domain>Service.ts`.** Views, components and
  composables call a service's function; nothing else calls `fetch`, opens a socket or imports
  `api.gen`. `npm run lint` fails when they do.
- A service owns the endpoint's path, its parameters and, where useful, how the answer is shown
  (a price formatted). It holds no application state.
- **The API types are generated from `api/openapi.yaml`, never written.** `npm run api:types`
  writes `src/services/api.gen.d.ts`; run it whenever the contract changes and commit both.
  `npm run lint` fails while the file is stale, and `npm run typecheck` then fails on every call
  the change broke.
- A service calls `request(method, path, { path, query, body })` from `api.ts`, with the path
  exactly as the contract spells it (`'/items/{id}'`), so the method, the parameters, the body and
  the answer are all checked against the contract. A named schema is `Schema<'ItemResponse'>`, a
  GET route's query is `Query<'/items'>`.
- Every answer is `{ data: … }` on success; a failure throws an `ApiError` with the status and the
  error envelope's `code` and `message`. A field Go leaves out when empty is absent, not `null`:
  read it with `?.` and `??`.
- In development the API is the site's own `/api`, which Vite passes to the `api` service; set
  `VITE_API_BASE_URL` when the built app finds it elsewhere.

## Routing

- Routes live in `src/router/`, named in kebab-case.
- `RouterLink` to navigate, never `window.location`.
- A route that needs a signed-in user is guarded in the router, not in its component.

## Deploy order

A screen that calls a new endpoint merges only once that endpoint is live in production. The app
and the API deploy separately, and nothing makes the app wait for the API: a screen that ships
first calls a route that is not there.

## Running it

`docker compose up -d` starts the app with the rest of the stack, at <http://localhost:5173>. With
the API already running, `npm run dev` in `web/` does the same outside Docker.

## Definition of done

- [ ] Every HTTP call goes through `src/services/`
- [ ] `npm run lint`, `npm run typecheck` and `npm run build` pass in `web/`
- [ ] The loading, empty and error states exist, and you have seen each one
- [ ] Every screen you touched you have seen on a phone's width and a desktop's
- [ ] Every component you added or moved sits in its domain's folder
