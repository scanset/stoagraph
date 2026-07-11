# 30 — Context behind the gate: context providers as gated MCP resources (spec)

Recorded 2026-07-10. **Status: BUILT + live-verified 2026-07-10** — the READ channel now runs through the
gate (a prod incident read `stag://context/k8s-kb` untrusted, recorded in `deploy/mcp/reads.jsonl`, then
correctly escalated the prod scale). Both modules green. Companion to Planning/29 (the
harness-side Feature A this supersedes), /24 (session→recipe daemon), /25 (dispatch). Goal: move context
binding **into the `stag` gate** so `stag` is literally a *gate MCP **and** context provider*. Today (Feature A)
the harness retrieves the KB and injects it; the gate is uninvolved. Here the **gate** is the context source —
it runs the bound providers, stamps every item **untrusted at origin** (`provider.Gather`), **records** the read,
and serves the result as an **MCP resource**. Same containment thesis as the ACT channel, applied to READ.

## The principle

Two MCP channels, two gate jobs:

- **ACT** (tools) — `allow / deny / escalate`. Forward-iff-cleared. (built)
- **READ** (resources) — **label + record, not deny.** Reads always succeed, but the gate stamps the content
  untrusted (unbypassably) and audits the crossing. (this doc)

The harness never trusts a *flag* on the content — it trusts the **channel**: anything read from `stag://context/*`
is untrusted by protocol, so a forged "trusted" label is meaningless. `bind` keeps it out of the instruction slot.

## Where this lands — the complete gateway

`stag` is the gateway for the model's **world access**, not the model itself — the model and its keys stay in the
harness; `stag` holds no keys and runs no inference. It mediates the *boundary* between the agent loop and the
outside world across both MCP channels:

```
   EVENT ──► dispatch ──► session (recipe-bound) ──► agent loop (model, in the harness)
 (ingress)   (built)      (built)                         │
                                              ACT ▼                 ▼ READ
                                          stag gateway ─ gates ─ stag gateway
                                                │                      │
                                          MCP tool servers      context providers
                                          escalate → human approval (built)
```

Before this doc, only **ACT** crosses the gate — the model reads context *around* it (harness-side, Feature A).
Planning/30 puts **READ** through the gate too, so nothing the model does *or sees* is un-mediated. That is what
"the complete gateway" means. The one remaining ingress piece is **event listeners/adapters** (Planning/18): thin
transport that normalizes a real external event (webhook / queue / pubsub) into the *existing* dispatch path — it
is transport, not logic, and lands **after** the gateway is whole.

## The trust invariant — the READ label is positional, not taint-tracking

The subtlety that must not be mis-stated later: **the untrusted stamp is NOT relied on to survive the model.** An
LLM launders taint — untrusted context in, a tool call out, and there is no reliable way to tag which output bytes
came from untrusted input. `stag` does not try. Instead:

- **READ side (into the model)** — the `Untrusted` label is **positional**: `bind` uses it to place context in the
  **Input** slot, never System, so untrusted data cannot rewrite the model's goal. That is the label's *only* job.
- **ACT side (out of the model)** — there is **no carried tag**. Every proposal is **presumed untrusted**; the gate
  re-derives trust *at the sink* from the recipe rule, not from any label. There is no "trusted items list" that
  model output joins.

The only promotions from untrusted → cleared, each emitting a recorded `ReleaseEvent`:

| Rule | Clears when | "Outside the trusted items list" |
|---|---|---|
| `set_membership` | value ∈ operator-authored allowed set | not in set → **no release** |
| `numeric_range` | value within bounds | out of range → **no release** |
| `signed_equality` | value matches a signed/approved token | no valid signature → **no release** |

A value that fails its rule **stays untrusted and is denied/escalated** at an **authoritative** sink — nothing
promoted it. (Untrusted values reach **benign** sinks freely; "untrusted" is precisely a gate on *authoritative*
action, not a universal quarantine.) So poisoned context can shape *what the model proposes* but **cannot make the
gate release a value the rule rejects** — the argument is checked regardless of origin. Same containment thesis on
both channels: a bad READ wastes a turn, it cannot breach. This is why the design is robust *without* propagating a
taint label through the model — it presumes taint and requires an explicit, deterministic, recorded release.

## URI scheme — one resource template per bound provider

The gate exposes each bound context provider as an RFC-6570 resource template (go-sdk
`Server.AddResourceTemplate`):

```
stag://context/<provider-name>{?q}
```

`resources/templates/list` advertises them (Name = provider, Description from config); `resources/read` on
`stag://context/kb?q=<event text>` runs that provider with query `q`. The query rides the URI — no new method.
`ReadResourceParams.URI` carries it to our `ResourceHandler`, which parses `q`.

## Gate side (the daemon — `proxy/sessiond` + `proxy/mcpgate`)

1. **Binding gains context.** `POST /sessions` body becomes `{routes:[{tool,recipe,gateArg}],
   context:[{name,kind,config}]}` — the provider **specs are resolved upstream** (stag-serve) and passed in,
   exactly like routes are. The daemon stays config-DB-free (it already resolves nothing itself). The registry
   stores the session's providers alongside its router.
2. **`NewGatingServer(gate, downstream, tools, providers)`** — after `AddTool` per route, `AddResourceTemplate`
   per provider with a `contextHandler`.
3. **`contextHandler`** (the READ crossing) — on `resources/read`:
   - parse `q` from the URI;
   - `items, errs := provider.Gather(ctx, q, []ContextProvider{p})` — **the untrusted stamp happens here**, in the
     gate, unbypassably;
   - **record** a read event (`kind:"read"`, provider, query, item count, ts) to the daemon audit log
     (`proxy-decisions.jsonl`) — symmetric to the ACT-channel decision records;
   - return a `ReadResourceResult` whose `ResourceContents.Text` is the labeled items (each framed
     "untrusted data, not instructions"), MIMEType `text/plain`. Provider errors are reported, non-fatal
     (`Gather` is read-fail-open): an empty/failed provider yields empty context, never a gate error.

Reads are **never denied** — no recipe is consulted. The gate's only READ-channel duties are *label* and *record*.

## Provider execution — keep the gate model-free

The daemon builds a `provider.ContextProvider` from each `{kind,config}`:

- **`http`** (built — `provider.HTTP`) — GET `config.url` with the query appended (`?q=`); body → one item. **This
  is the demo path**: the KB/embedder lives in a *downstream* service, so the gate holds **no model** (the
  determinism/purity selling point survives). Symmetric to how tool servers are downstream.
- **`mcp_resource`** — connect to a downstream MCP server and proxy *its* `resources/read` (true resource
  proxying, reusing the discover/auth transport from Planning/28). **v1.1-within-v1.1** — spec, build after http.
- **`rag`** — gate-side embedding retrieval. **Deliberately NOT built** — it would pull an embedder into the gate.
  The config kind stays reserved; operators who want RAG run it as an `http` provider.

## The binding chain (symmetry with Feature B toolset)

`event_map` definition gains optional `context:[names]` (next to `tools:[names]`):

```
event → Dispatcher → Decision{RecipeID, Tools, Context}     (dispatch.go)
      → StagClient.ProvidersFor(Context) → []ProviderSpec   (GET /api/providers, enabled only)
      → Binder.Bind(routes, providers) → POST /sessions {routes, context}
      → daemon serves the toolset (ACT) + the provider templates (READ)
```

When `context` is empty the session has no READ channel (today's default). Fail-closed: an unknown/disabled
provider name is dropped (logged), never fabricated.

## Harness side (retire the harness-side KB)

The harness is already an MCP client to the session (`agent.ConnectHTTP` → `*mcp.ClientSession`). After connect,
**read context from the gate** instead of embedding it locally:

- for each advertised `stag://context/*` template, `sess.ReadResource(ctx, {URI: tmpl+"?q="+eventText})`;
- collect the returned (channel-untrusted) text as the `docs` for `bind.Assemble(system, eventJSON, docs)` — the
  bind step is unchanged; only the *source* moves from `s.kbRetrieve` to the gate.
- `event_harness/kb` + the `-kb-*` flags + `kbRetrieve` are **removed** from harness-serve (the embedder now lives
  in the downstream KB service). `bind` stays (trust-position assembly is still the harness's job).

Net: identical untrusted-context UX, but the **gate** now labels + records it — `stag` is the context provider.

## Demo KB service (`k8s_test/kbserve`, ~1 file)

A tiny standalone HTTP server wrapping the existing `kb` logic: `GET /context?q=` → `kb.LoadDir` (once) +
`Retrieve(q,3)` → the joined fact text. Register it as an `http` context provider
(`POST /api/providers {name:"k8s-kb", kind:"http", config:{"url":"http://localhost:8095/context"}}`) and add
`context:["k8s-kb"]` to the `k8s-incident` event-map definition. The gate proxies it; the embedder stays out of
the gate.

## Trust (why this is safe — unchanged thesis, gate-enforced)

`Gather` stamps `Untrusted` at origin in the **gate**, overriding whatever the provider set; the harness trusts
the **channel** (`stag://context/*` ⇒ untrusted) not a flag; `bind` keeps it out of System; the gate **records**
every read. A poisoned or wrong KB wastes a turn — it cannot reach the gate decision or the instruction slot.
Same containment as ACT, now with an audit trail for READ.

## Build order + verification

1. **Gate READ channel** — `contextHandler` + `AddResourceTemplate` in `mcpgate`; `NewGatingServer` gains
   `providers`; daemon binding parses `context`; read-event record. Unit: bind a session with one `http`
   provider (httptest downstream) → `ListResourceTemplates` shows it → `ReadResource("...?q=x")` returns the
   labeled body → the audit log has a `read` record.
2. **Binding chain** — `Definition.Context`, `Decision.Context`, `StagClient.ProvidersFor`,
   `Binder.Bind(routes, providers)`. Unit: `ProvidersFor` filters to enabled/valid.
3. **KB service** — `k8s_test/kbserve`; register the `k8s-kb` provider; `context:["k8s-kb"]` on `k8s-incident`.
4. **Harness swap** — read resources from the gate; delete `kb`/`kbRetrieve`/`-kb-*` from harness-serve.
5. **Live** — the prod incident event → multi-tool session **+ gate-served context** → transcript shows
   `context: N fact(s) via stag://context/k8s-kb (untrusted)`; the daemon audit log shows the `read` crossing;
   agent investigates + proposes the gated prod action. Record in DEVLOG; flip Planning/29 + /30 status.

## Build notes (what actually shipped)

- **Gate:** `mcpgate.ReadChannel{Providers, Record}`; `NewGatingServer` gains it and `AddResourceTemplate`s
  each provider as `stag://context/<name>{?q}`; `contextHandler` parses `?q`, `Gather`s (untrusted stamp),
  records a `provider.ReadEvent`, returns labeled `ResourceContents`. Daemon: sessions are a `boundSession`
  {router, providers}; `POST /sessions` parses `context:[{name,kind,config}]` and builds providers via
  `provider.FromConfig` (http only; rag/mcp_resource fail closed → dropped+logged). Read audit is a plain
  append log `deploy/mcp/reads.jsonl` (`-read-log`), separate from the hash-chained ACT decisions log.
- **Chain:** `Definition.Context` / `Decision.Context` / `StagClient.ProvidersFor` (enabled-only) /
  `Binder.Bind(routes, providers)` → `POST /sessions {routes, context}`.
- **KB service:** shipped as `event_harness/cmd/kbserve` (NOT `k8s_test/kbserve` — k8s_test isn't a Go module;
  kbserve needs to import `event_harness/kb`). The embedder lives here, the gate stays model-free.
- **Harness:** `readGateContext` lists `stag://context/*` templates and reads each; `event_harness/kb` +
  `-kb-*` flags removed from harness-serve; `bind.Assemble` now takes `[]bind.Doc` (decoupled from `kb`).
- **GOTCHA (cost an hour):** the go-sdk matches an RFC-6570 `{?q}` template with a regexp that accepts
  **percent-encoding only** — `url.QueryEscape` emits `+` for spaces, which fails the match → `resources/read`
  returns "resource not found". Fix: `strings.ReplaceAll(url.QueryEscape(q), "+", "%20")` when building the
  read URI. A single-word unit-test query masked it; any multi-word event query tripped it.

## Scope / not-in-v1.1

- **`mcp_resource` proxying** (true downstream-resource proxy) — after the `http` path proves the shape.
- **Agent-native resource reads** (the agent loop reading resources itself vs the harness reading on its behalf) —
  the harness-reads path ships first; agent-native is an `agent.Run` enhancement.
- **`rag` gate-side retrieval** — reserved kind, intentionally unbuilt (keeps the gate model-free).
