# 18 — Adapters: the MCP-server + context-provider proxy (forces everything through)

Recorded 2026-07-03. **Decision (Curtis): build the Adapters slice next.** This is the capability from the
Planning/17 refinement — stag as a proxy for **both** channels of an agent's I/O: the **ACT** channel
(MCP tool servers → gated) and the **READ** channel (context providers → labeled untrusted + recorded).
Configuring these adapters is the admin console's **Adapters** tab; the config lives in the SQLite store this
slice introduces.

## Two adapter families, one discipline

Both are the same adapter pattern already used across the codebase (`model/claude`, `model/openai`,
`proxy/mcpgate`): one interface, per-protocol implementations, third-party deps quarantined, the core clean.

**ACT — MCP tool servers (gated).** The admin connects downstream MCP servers; stag proxies their tools
and gates each call (Slice 0, U22, is the runtime; this slice is connecting/configuring them and binding
tools → recipes). A downstream server is `{name, transport: stdio-command | http-url, tools[]}`.

**READ — context providers (labeled + recorded).** Different sources speak different protocols, so the read
side is an adapter behind one interface:

```
type ContextProvider interface {
    Provide(ctx, query) ([]ContextItem, error)   // ContextItem{Source, Text, Trust: untrusted, Score}
}
```
- Adapters: `RAGProvider`, `MCPResourceProvider`, `HTTPProvider`, … (each quarantined if it pulls a dep).
- **Load-bearing rule:** every adapter returns items stamped **untrusted at origin** (the trust-position
  invariant, U15, generalized). No adapter can hand back "trusted" context. The read proxy's job is
  label-at-origin + record — the mandatory chokepoint, not heavy read-time denial (Planning/17 asymmetry).

## The SQLite config store (entities + their relationships)

The relational win is the **bindings**, not the recipe text (recipes stay YAML — a file or a text column).
`modernc.org/sqlite` (**pure Go, no cgo**), plain `database/sql`, portable SQL so Postgres later is a driver
swap. Schema (indicative):

```
recipe(name PK, yaml TEXT, valid, hash, updated_at)          -- or keep recipes as files; store metadata only
mcp_server(id PK, name, transport, target, enabled)
mcp_tool(server_id FK, name, input_schema)                   -- discovered from the server's tools/list
context_provider(id PK, name, kind, config JSON, enabled)
route(tool_name, recipe_name FK, gate_arg)                   -- tool -> recipe binding (the router)
provider_binding(provider_id FK, recipe_name FK)             -- which providers a policy's rules may read
recipe_dep(parent FK, child FK)                              -- recipe composition graph (Planning/19)
```

The **route** and **binding** tables are exactly the "relationships of recipes to these items" — the graph
the Adapters/Policies tabs render, and the source the live gate builds its `proxy.Router` from.

## Wiring the gate to the store

Today the live `/api/decide` gate loads ONE recipe (`-recipe` flag). This slice makes it **multi-tool**: the
router is built from the `route` table (tool → recipe → gate_arg), so adding a tool binding in the console
adds it to the gate. This is Planning/17 broadening #1 (the tool→recipe router), landing with Adapters.

## Admin console — the Adapters tab

- **MCP servers:** add/edit `{name, transport, target}`; on connect, list the server's tools; bind each
  tool → a recipe + gated arg (a `route` row).
- **Context providers:** add/edit `{name, kind, config}`; bind to recipes.
- Both show enabled/health state. This is where "forces everything through stag" is configured.

## Build sequence

1. **SQLite store** — `modernc.org/sqlite` (quarantined), a `store` package with typed CRUD (ladder-built;
   the load-bearing bit is fail-closed queries + the route-table → `proxy.Router` build). **No migrations:
   one DDL file per schema — edit it and re-init; recollecting data is fine** (project rule).
2. **MCP-server adapter (admin side)** — connect a downstream (reuse the SDK client), `tools/list`, persist
   tools; bind tool → recipe.
3. **`ContextProvider` interface + one adapter** — start with an MCP-resource provider (reuse the SDK) or a
   RAG provider (reuse `kb`); all items untrusted.
4. **Adapters console page** — CRUD UI for servers + providers + bindings.
5. **Gate-loads-from-store** — build `proxy.Router` from `route`; the live gate goes multi-tool.

## Honest scope

- **No auth, single deployment** (internal). SQLite is single-writer — fine for one admin process; Postgres
  when multi-writer/multi-tenant.
- The **stdio MCP proxy** (`stag-mcp-proxy`, so a real agent connects) stays deferred — this slice is the
  admin/config surface + the HTTP gate, not the agent-facing stdio server.
- Read-proxy v1 is **label + record**; policy-filtering of context (allow/deny a provider, redact) is a
  refinement.

## Status

**BUILT 2026-07-03 (U25–U27).** The SQLite store (U25), the store-driven multi-tool gate (U26, route → router),
and the Adapters config surface (U27: MCP-server discovery + context providers + the `/adapters` console page)
all shipped, PRODUCTION-CLEAN. The console now configures **both** proxy channels and the gate runs off the
saved route table. **Deferred:** the *deep* read-proxy (context actually feeding the gate decision — ties to
the recipe model consuming context, Planning/19, and the Taint Map UX, Planning/20); a runnable stdio proxy so
a real agent connects. Pairs with Planning/17 (the proxy architecture) and Planning/19 (composition + foreach).
