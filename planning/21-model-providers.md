# 21 — Model providers: connect Claude and OpenRouter (the intelligence source)

Recorded 2026-07-04. stag can now gate tool calls and context (dual proxy) and author foreach/composed
recipes. To **use** a model as an intelligence source — the proposer side (U9–U12: `model.Proposer`, the
Claude adapter U10, the OpenAI-compatible adapter U11 that already covers OpenRouter/ollama/vLLM/OpenAI) — a
user must be able to **connect a model** from the console: pick the dialect, base URL, model id, and where its
API key lives. This is the model-provider config surface, the fourth adapter type alongside MCP servers, context
providers, and routes.

## Secret handling

Two ways to supply a key, per row (Curtis, 2026-07-04 — direct storage requested for dev/testing):

1. **Stored directly** — `api_key` holds the secret in the SQLite file, **unencrypted** (acceptable in
   dev/testing). This is the default console flow: paste the key. `api_key` wins when both are set.
2. **Env-referenced** — leave `api_key` blank and set `api_key_env` to the NAME of an environment variable the
   server resolves with `os.Getenv` at call time (matches the file `config.Proposer{APIKeyEnv}`, U13). Keeps
   the secret out of the DB for operators who prefer that.

**The API never echoes a stored key.** `GET /api/models` returns `keyPresent` (a usable key exists — stored or
via a set env var) and `keyHint` (a masked tail, `…abcd`) so the console can show configured/missing and tell
keys apart, but never the value itself. An edit that omits `apiKey` preserves the key already on file (so
changing the model id doesn't blank the key). **Caveat:** unencrypted at rest — fine for local dev, NOT for a
shared/production deployment; an at-rest encryption or secret-manager option is a later hardening step.

## The config shape

One table (`model_provider`), edited-in-place per the no-migrations rule:

| field         | meaning                                                            |
| ------------- | ----------------------------------------------------------------- |
| `name`        | operator label, primary key (grammar-sanitized)                    |
| `kind`        | `claude` (Anthropic Messages API, SDK adapter) or `openai` (OpenAI-compatible: OpenRouter, ollama, vLLM, OpenAI) |
| `base_url`    | endpoint. openai: required (e.g. `https://openrouter.ai/api/v1`). claude: optional override (blank = SDK default) |
| `model`       | model id (`claude-opus-4-8`; `anthropic/claude-opus-4` on OpenRouter)               |
| `api_key_env` | env-var NAME holding the key (`ANTHROPIC_API_KEY`, `OPENROUTER_API_KEY`)            |
| `enabled`     | on/off                                                            |

## Scope of this unit (U31)

Config + console only — **connect and manage** providers; **do not invoke** them yet (no live prompts). The
store CRUD, the `/api/models` endpoints (with the `keyPresent` check), and the `/models` console page. The
bridge from a stored provider to a live `model.Proposer` (a factory that resolves the key from env and builds
the Claude or OpenAI adapter), and where the proposer plugs into the gate (broker/Decide — "propose then gate",
incl. a foreach list of candidates), is the **next** unit. Building the connection surface first lets the
operator wire keys without any request being sent.

## Later (the "use" unit, not this one)

- A factory `provider → model.Proposer` (openai adapter for `openai`; claude adapter for `claude`), key from
  `os.Getenv`. Kept OUT of the `serve` package if it would pull the anthropic SDK in (quarantine); more likely
  it lives in the broker/runner path.
- Broker wiring: a route (or a console action) that runs the configured proposer to generate a candidate (or a
  candidate list for foreach), then gates it through the recipe. Deterministic gate, untrusted proposal — the
  model never sits in the enforcement path.
- A "test connection" affordance (a minimal, opt-in round-trip) once the user is ready to send prompts.
