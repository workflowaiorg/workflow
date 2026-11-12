import type { components, paths } from './api.gen'

// The types below read api.gen.d.ts, so services never have to. A service only uses `request`,
// `Schema`, `Query` and `ApiError`; the rest is how `request` learns, from the path and the method
// alone, what the call takes and what it answers with.

// A named shape from the contract: Schema<'HealthStatusResponse'>.
export type Schema<Name extends keyof components['schemas']> = components['schemas'][Name]

type Path = keyof paths
type Method = 'get' | 'post' | 'put' | 'patch' | 'delete'

// The query string a GET route reads: Query<'/items'>.
export type Query<P extends Path> = paths[P]['get'] extends { parameters: { query?: infer Values } }
  ? NonNullable<Values>
  : never

// The methods a path really has. The generated file lists every method on every path and marks
// the missing ones `never`; this keeps only the real ones, so request('put', '/health') fails.
type MethodOf<P extends Path> = {
  [M in Method]-?: [NonNullable<paths[P][M]>] extends [never] ? never : M
}[Method]

// One route: one path with one method.
type Operation<P extends Path, M extends MethodOf<P>> = NonNullable<paths[P][M]>

// The values for `{id}` and the like in the path, when the route has any.
type PathParams<Op> = Op extends { parameters: { path: infer Values } }
  ? [Values] extends [never] | [undefined] ? { path?: never } : { path: Values }
  : { path?: never }

// The query string, when the route reads one.
type QueryParams<Op> = Op extends { parameters: { query?: infer Values } }
  ? [Values] extends [never] | [undefined] ? { query?: never } : { query?: Values }
  : { query?: never }

// The body: JSON, or FormData for an upload, or nothing.
type Body<Op> = Op extends { requestBody?: { content: infer Content } }
  ? Content extends { 'application/json': infer Json } ? { body?: Json }
  : Content extends { 'multipart/form-data': unknown } ? { body: FormData }
  : { body?: never }
  : { body?: never }

type Options<Op> = PathParams<Op> & QueryParams<Op> & Body<Op>

// The options argument may be left out only when nothing in it is required.
type Args<Op> = object extends Options<Op> ? [options?: Options<Op>] : [options: Options<Op>]

// What the route answers with on success (200 or 201).
type Success<Op> = Op extends { responses: infer Responses }
  ? {
      [Status in keyof Responses]: Status extends 200 | 201
        ? Responses[Status] extends { content: { 'application/json': infer Json } } ? Json : never
        : never
    }[keyof Responses]
  : never

// A failure the API answered with: its HTTP status and the code and message of its error envelope.
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message)
  }
}

// Where the API is: this site's /api unless the build says otherwise.
const baseURL = import.meta.env.VITE_API_BASE_URL ?? '/api'

// Calls one route of the contract, with the path exactly as the contract spells it
// ('/items/{id}'), and answers with its success body; a failure throws an ApiError.
export async function request<P extends Path, M extends MethodOf<P>>(
  method: M,
  path: P,
  ...[options]: Args<Operation<P, M>>
): Promise<Success<Operation<P, M>>> {
  // The types above have checked the options against the route; here they are plain values.
  const { path: values, query, body } = (options ?? {}) as {
    path?: Record<string, string | number>
    query?: Record<string, unknown>
    body?: unknown
  }
  let url = baseURL + path.replace(/\{(\w+)\}/g, (_, name: string) => encodeURIComponent(String(values?.[name])))
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(query ?? {})) {
    for (const one of Array.isArray(value) ? value : [value]) {
      if (one !== undefined) search.append(key, String(one))
    }
  }
  if (search.toString()) url += `?${search}`

  const init: RequestInit = { method: method.toUpperCase() }
  if (body instanceof FormData) {
    init.body = body
  } else if (body !== undefined) {
    init.body = JSON.stringify(body)
    init.headers = { 'Content-Type': 'application/json' }
  }
  const response = await fetch(url, init)
  const answer = await response.json().catch(() => undefined)
  if (!response.ok) {
    const error: Schema<'ErrorResponse'>['error'] | undefined = answer?.error
    throw new ApiError(response.status, error?.code ?? 'unknown', error?.message ?? response.statusText)
  }
  return answer
}
