## Agents

An agent is a service under `agents/` that calls a language model: a call brings everything it
needs, the agent asks the model, and the answer carries everything it made. Agents answer over
A2A, reach the model only through `pkg/llmgateway`, and import no database, queue or other
service; `agents/boundary_test.go` fails when one does. The `llm-agents` skill says how.
