# 24 — stag-proxy: the standing gating MCP server (the daemon an agent connects to)

Recorded 2026-07-04. The mediation-half's final piece (Planning/22 #1, /23 sequencing).

**Status: v1 BUILT (2026-07-04). v2 BUILT (2026-07-05).** v1: `cmd/stag-proxy` serves the gating MCP server over
stdio from store config; proven e2e (`cmd/stag-proxy/e2e_test.go`). v2: `-http` runs the standing daemon —
`proxy/sessiond` binds each MCP session to a dispatcher-chosen recipe over streamable HTTP; proven e2e
(`proxy/sessiond/sessiond_test.go`: two sessions bind the same tool to different recipes → the same call is
allowed in one and denied in the other; one shared-log crossing; unknown token → 400). stdio mode unchanged.
Remaining: the dispatcher that mints sessions (Planning/25, event_harness), harness→daemon migration, multiple
downstreams, stateful traversal (v3).

## What it is

A binary `cmd/stag-proxy` that is, at once:
- an **MCP server** to the agent (presents the governed tools, gates every call), and
- an **MCP client** to the real downstream MCP servers (forwards only cleared calls),

with the deterministic `proxy.Gate` in the middle. `NewGatingServer(gate, downstream, tools)` is the core; the
daemon is the wiring around it.

## Wiring (all pieces already exist except the daemon + transport)

1. **Config from the store** — `store.ListMCPServers` (which downstreams to proxy + their discovered tools) and
   `store.ListRoutes` (tool → recipe → gated arg). Same store `stag-serve` manages, so the console configures the
   proxy live.
2. **Build the gate** — `router.Build(specs, recipes.Get)` → `proxy.Router` → `proxy.Gate{Routes, Sink}` with the
   egress sink (hash-chained/signed audit). Identical to `serve.liveGate`.
3. **Connect downstream** — for each enabled `mcp_server`, dial it as an MCP client (`mcpgate` transport:
   stdio→CommandTransport, http→StreamableClientTransport) → a `*mcp.ClientSession`.
4. **Stand up the gating server** — `NewGatingServer(gate, downstream, tools)` presenting the downstream tools;
   each `tools/call` is gated, cleared calls forwarded (denied ones return a tool error, never reach downstream).
5. **Serve it over a transport the agent connects to** (below).

## Transport

- **Streamable HTTP (recommended for the daemon):** a standing endpoint many agent hosts connect to. Each
  connection is a session — the natural seam for session→recipe (below).
- **stdio (for a single subprocess agent):** the agent spawns `stag-proxy` and speaks over its stdio (like the
  pii-demo server). Simple; one agent per process. Good for the first e2e test.

Build stdio first (trivial to test with an MCP client), add streamable HTTP for the real daemon.

## Recipe selection — v1 vs the session→recipe target

- **v1 (buildable now): per-call via the route table.** The gate resolves `tool → recipe` from `store` routes.
  Every call the connected agent makes is gated by its tool's recipe. This is exactly today's `/api/decide`
  semantics, now over MCP. No new mechanism — ship this first.
- **v2: session → recipe (Planning/22).** Bind a whole session to one recipe by *which endpoint/session the agent
  connected to* (`/mcp/support` vs `/mcp/refunds`, or a session tag). The daemon selects the governing recipe per
  connection. This is where "the event's recipe" attaches without the model touching it.
- **v3: per-session stateful traversal** — the session tracks position in the recipe graph; each call is a legal
  transition. Makes "walk the whole recipe" literal. Deferred (stateful).

## Multiple downstreams

`NewGatingServer` forwards to ONE downstream session. Real use has several `mcp_server` rows. v1: connect each
enabled server, aggregate their tools, and route each `tools/call` to the owning downstream (the `mcp_tool` table
already maps tool→server). Either extend `NewGatingServer` to take a tool→session map, or wrap it. If only one
server is configured, the single-downstream path is enough for the first cut.

## Fail-closed posture (unchanged invariants)

- A tool with no route → unrouted → **deny** (never forwarded). An unreachable downstream → its tools are
  absent/denied. A malformed arg → empty value → fails the rule → deny. `Gate.Decide` is the only authority.
- The daemon holds **no model and no keys** — it only gates + forwards. Context reads are proxied + labeled
  untrusted by `provider` (the READ channel) the same way.

## v1 scope (the unit to build)

`cmd/stag-proxy`: load store config → build gate → connect the (single, first) downstream → `NewGatingServer` →
serve over **stdio**. An MCP client connecting to it sees the governed tools; a cleared call is forwarded and
returns the downstream result; a denied call returns a gate error and never reaches downstream; the crossing is
recorded. Test e2e with an in-memory/stdio MCP client against a mock downstream (the pii-demo server). Then:
streamable HTTP + session→recipe (v2).

## Relationship to what exists

- Reuses: `proxy` (Gate), `proxy/mcpgate` (NewGatingServer, transports, Discover), `router`, `store`,
  `recipestore`, `egress`. No new dependency.
- Complements `stag-serve` (console): same store, same gate, different front door (MCP for agents vs HTTP for
  operators). Could even be one binary with two listeners, but a separate `cmd/stag-proxy` keeps concerns clean.

---

## v2 spec (2026-07-05): standing daemon + session→recipe

The build unit. Delivers Planning/22 #1 and the mediation half of Planning/25. Goal: ONE long-lived process
serving streamable HTTP, where **each MCP session is bound to a dispatcher-chosen recipe/policy**, and ONE
process owns ONE audit log (which retires the v1 shared-log fork by construction).

### Transport
`stag-proxy -http :addr` runs the daemon via `mcp.NewStreamableHTTPHandler(getServer, opts)`. The existing stdio
mode (`-downstream X`, no `-http`) stays for backward compat + the current e2e test. The daemon dials its
downstream(s) ONCE at startup; the connection(s) and the egress sink are SHARED across all sessions.

### The session→recipe seam
`getServer(req *http.Request) *mcp.Server` is called by the SDK once per NEW MCP session — the exact hook we need:
1. **Bind (trusted, out-of-band):** the dispatcher calls `POST /sessions {routes:[{tool,recipe,gateArg}]}`. The
   daemon builds a `proxy.Router` via `router.Build` (fail-closed: a bad/missing recipe rejects the binding),
   mints an **opaque token** (crypto/rand), stores token→router in an in-memory registry, returns
   `{token, path:"/mcp/<token>"}`.
2. **Connect (the untrusted agent):** the dispatched agent connects to `http://daemon/mcp/<token>`. `getServer`
   pulls the token from the path, looks up its router, and returns
   `mcpgate.NewGatingServer(Gate{Routes: sessionRouter, Sink, Approvals, OnEscalate}, downstream, tools)`.
   Unknown/absent token → return nil → the SDK serves 400 (fail closed).

**The trust boundary:** the untrusted agent CANNOT choose its own recipe — the token is minted by the dispatcher
and bound server-side; the agent holds only an opaque reference. This is the whole point of session→recipe: the
event's recipe attaches to the session, not to anything the model touches. (Contrast v1: a global tool→recipe
table every connection shares.)

### Session entity (Planning/25)
The registry IS the Session entity for v1: token → {routes} — one entry recipe's routing, in-memory, ephemeral
(lost on daemon restart; a dropped session just re-dispatches). `recipe_trace`/`children` stay singular/empty
until runtime recipe→recipe and subagent nodes land. Do not persist yet.

### Fail-closed (unchanged)
Unknown token → 400; a session calling a tool its router doesn't cover → deny (unrouted); a recipe that won't
build at bind time → the binding is rejected, not silently dropped. `Gate.Decide` remains the only authority; the
daemon holds no model, no keys.

### Scope of THIS build
Daemon (`-http`) + `POST /sessions` control API + the `getServer` token→router seam + a shared downstream/sink +
an e2e test: create two sessions binding the SAME tool to DIFFERENT recipes → each session gates by its own; an
unknown token → 400; a cleared call forwards to the (mock) downstream and records ONE crossing on the shared log.
Keep stdio mode green.

**Out of this build (later, component 1):** the dispatcher that mints sessions from events, and migrating the
harness console to connect to the daemon (today it spawns stdio). Multiple downstreams and stateful traversal (v3)
remain deferred.
