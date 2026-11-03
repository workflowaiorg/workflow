---
name: llm-agents
description: "How an agent that calls a language model is built: the A2A service under agents/, its card, the model gateway through pkg/llmgateway, structured answers, what an agent may import, and testing without a real model."
---

# Skill: LLM agents

An agent is a service that calls a language model as its main work. It answers over
[A2A](https://a2a-protocol.org) with [a2a-go](https://github.com/a2aproject/a2a-go), and calls the
model through `pkg/llmgateway`, which uses the Responses API of the
[OpenAI Go SDK](https://github.com/openai/openai-go). `agents/summary` is the example to copy.

## When this skill applies

- Adding an agent, or changing what one does
- Changing a prompt, a model call, or the answer an agent sends back
- Calling an agent from another service
- Testing an agent

## Payload in, payload out

**A call brings everything the agent needs, and the answer carries everything it made.** An agent
keeps no state and reaches nothing but its model: no database, no queue, no other service, and no
tool the model could call that reaches one. The service that calls it loads what the agent needs
and saves what it answers.

An agent imports, from the project, only its own packages and `pkg/errors`, `pkg/logging` and
`pkg/llmgateway`. `agents/boundary_test.go` fails `go test ./...` when one imports anything else of
the project, or depends on a database, Redis or gRPC client, directly or through a library. Work that needs to read or write data is not an
agent's: it belongs to a service, which passes the data in.

## Layout: `agents/<name>/`

| Path | What it is |
|---|---|
| `cmd/a2a/main.go` | starts the server and stops it on SIGTERM, as a service's `main` does |
| `internal/config/` | `PORT`, `AGENT_URL` and the model's `LLM_API_KEY`, `LLM_BASE_URL`, `LLM_MODEL`; `main` exits when `LLM_MODEL` is unset |
| `internal/route/` | `Register(cfg)`: the card and the call path, wired with the gateway, controller and handler |
| `internal/handler/<domain>/a2a/` | the `a2asrv.AgentExecutor`: reads the call, calls the controller, writes the answer |
| `internal/controller/<domain>/` | the work: builds the prompt, calls the model through its `Gateway` interface |
| `internal/gateway/llm/openai/` | the model, through `pkg/llmgateway` |

There is no Gin in an agent: `net/http` serves the two handlers a2a-go gives. A new agent copies
`agents/summary`, and gets its own service in `docker-compose.yaml`, with its own port.

## The card

`route.card` says what the agent does, at `/.well-known/agent-card.json`: its name, the URL of its
calls (`AGENT_URL` + `/invoke`, from config, never written in code), and one skill per kind of
work `Execute` does, with the fields it takes and answers. The card says only what the agent does.

## The handler

`Execute` takes the call's first part as a `DataPart` of named fields, and answers one `DataPart`
of flat fields: never JSON inside a string.

- Check that the message has a part, that it is a `DataPart`, and that each field the call needs is
  there with its type, before using it. A JSON number arrives as `float64`.
- Every answer carries what it cost, `prompt_tokens`, `completion_tokens` and `total_tokens`, and
  the `model`, beside its own fields.
- A call that is wrong, a missing field or a `CodeInvalidRequest` from the controller, is yielded
  wrapping `a2a.ErrInvalidParams`; any other failure is logged, and yielded as the `APIError`'s
  message wrapping `a2a.ErrInternalError`. The caller gets the error's text, so the cause never goes
  in it.
- No logic: the work is the controller's.

## The controller

It takes the fields the call brought and returns what it made, with the `llmgateway.Usage` of
every model call it made, added up with `Add`. It defines the `Gateway` interface it needs; it
never imports the SDK. Its instructions to the model are constants at the bottom of the file.

## The gateway and `pkg/llmgateway`

A gateway holds a `*llmgateway.Completer` and never calls the SDK itself. `pkg/llmgateway` turns
every failure into an `APIError` with `CodeBadGateway`, refuses an answer the provider cut short,
and returns the tokens each call cost.

- **`CompleteText`** returns plain text. An empty answer is not an error there: the gateway decides
  what one means for its agent, as `agents/summary`'s refuses it.
- **`Complete`** returns JSON matching a JSON Schema, for an answer code reads. The answer's struct
  is in `internal/model/`, its schema a package-level variable beside the gateway method, and
  JSON that does not decode into the struct is a `CodeBadGateway`.

Changing provider is a change of `LLM_BASE_URL` and `LLM_MODEL`, never of code: any provider that
serves the Responses API works, a local one with no `LLM_API_KEY` too. Locally they are in `.env`,
which git ignores; `.env.example` beside it lists them, and a new setting goes in both.

## Calling an agent

A service calls an agent through a gateway, `internal/gateway/<domain>/a2a/`, with
`a2aclient.NewFromCard` and the card `agentcard.NewResolver(...).Resolve` fetched from the agent's
URL. The gateway sends a `DataPart`, reads the answer's fields, and returns an `APIError`, never
an a2a-go error: `errors.Is(err, a2a.ErrInvalidParams)` is a `CodeInvalidRequest`, any other a
`CodeBadGateway`.

## Testing

No test calls a real model: a real model costs money and answers differently each time.

- The gateway and `pkg/llmgateway` are tested against an `httptest` server that answers as the
  Responses API does, with `LLM_BASE_URL` pointed at it.
- The controller is tested with a stand-in `Gateway`, checking the prompt it sends and what it
  makes of the answer.
- The handler is tested by building an `a2asrv.ExecutorContext` and reading what `Execute` yields:
  a good call, each missing or wrong field, and a failing controller.
- The route is tested end to end: the card, then a call through `a2aclient`, against the stand-in
  provider.

## Definition of done

- [ ] The agent takes everything in its call and answers everything it made; it imports nothing
  `agents/boundary_test.go` forbids
- [ ] The model is called through a gateway on `pkg/llmgateway`; the controller defines the
  interface it needs and never imports the SDK
- [ ] The answer is flat `DataPart` fields, with the tokens it cost and the model
- [ ] The card's skills are the work `Execute` does, and its URL comes from config
- [ ] The agent has its service in `docker-compose.yaml`
- [ ] The handler, controller, gateway and route have tests, and none calls a real model
