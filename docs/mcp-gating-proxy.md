# The stag MCP gating proxy

`stag-proxy` sits between an AI agent and the MCP tool servers it calls. To the agent it **is** an MCP
server (it presents the tools); to your real MCP servers it is an MCP **client** (it forwards calls).
Between them is a deterministic gate: every `tools/call` is evaluated against a recipe (a policy), and
**only a cleared call is forwarded** to the real server. A denied or escalated call returns a tool
error and never reaches the downstream.

No model runs in the enforcement path — the decision is a deterministic walk of the recipe graph. The
gate holds no model and no API keys.

## The guarantee

- **Complete mediation** — every governed tool call is gated; there is no path to the downstream that
  bypasses the gate.
- **Forward-iff-cleared** — a call is forwarded only when the recipe verdict is `allow`. `deny`,
  `escalate`, and any fault are never forwarded.
- **Fail closed** — an unrouted tool, a missing/malformed argument, an unreachable downstream, or a
  recipe that will not parse all result in denial, never a silent allow.
- **Tamper-evident audit** — every cleared crossing is appended to a hash-chained, optionally-signed
  log.

## Two ways to run it

### stdio — a single agent (e.g. Claude Desktop)

The agent spawns `stag-proxy` and speaks MCP over its stdio. Tool→recipe bindings come from the config
store (managed in the console).

```
stag-proxy -downstream <your-mcp-server> -store <config.db>
```

Point your MCP client at that command — e.g. `claude_desktop_config.json`:

```json
{ "mcpServers": { "stag": { "command": "stag-proxy", "args": ["-downstream", "my-server"] } } }
```

### daemon — many sessions, session→recipe (streamable HTTP)

A standing server. A trusted controller binds a session to a recipe and hands the agent an opaque
endpoint; the agent cannot choose its own recipe.

```
stag-proxy -http :8091          # fronts every enabled downstream; each route names which server serves it
```

- `POST /sessions {routes:[{tool,server,recipe,gateArg}]}` → `{token, path}` — the control plane (trusted).
- The agent connects to `/mcp/<token>` (streamable HTTP); every call is gated by that session's recipe,
  and the session's `tools/list` shows only the tools that recipe governs.
- An unknown token → `400` (fail closed). Idle sessions are closed after 30 minutes.

## What a refusal looks like

A gated-but-denied call returns an MCP tool error (`isError: true`) with a human message and structured
metadata in the protocol-reserved `_meta`:

```json
{ "isError": true,
  "content": [{ "type": "text", "text": "stag gate: deny — \"delete_namespace\" not forwarded" }],
  "_meta": { "stag": { "tool": "delete_namespace", "verdict": "deny" } } }
```

On an approval-gated `escalate`, `_meta.stag.approvalId` is set, so a controller can drive a
human-approval flow (approve → signed release → the retried call is forwarded).

## Both channels are gated

- **ACT — tools.** `stag-proxy` gates `tools/call`: **allow / deny / escalate**, forward-iff-cleared.
- **READ — resources.** Each bound context provider is served as an MCP resource template
  (`stag://context/<name>{?q}`). A `resources/read` runs the provider, stamps every item **untrusted at
  origin** (unbypassably), records the crossing to the read audit log, and returns it. **Reads are
  label+record — never denied**: no recipe is consulted, because a read cannot itself exercise authority.

  The untrusted stamp is **positional, not taint-tracking** — it keeps context out of the instruction
  slot. It is *not* relied on to survive the model; the ACT gate re-derives trust at the sink. See
  [SECURITY.md](../SECURITY.md).

## Control plane

Authenticated by default. Role tokens are generated (`0600`) into `data/control.tokens` on the
gate's first start; env vars (`STAG_*_TOKEN`) override for containers.

- `POST /sessions` (bind a session — it *chooses the recipe*) requires the **`dispatch`** role.
- `/mcp/<token>` takes **no** bearer: the opaque session token *is* the untrusted agent's credential.
  Handing the agent a control-plane bearer would be exactly backwards.
- Approving an escalation requires **`approve`**, which the orchestrator is never given.

## Scope (v1)

- **Scalar gated arguments.** A recipe gates named arguments (e.g. `namespace`, `replicas`), compared
  as strings; non-scalar arguments are stringified. Which arguments a tool's policy judges is set by its
  route — see [routes.md](routes.md).
- **Multi-server fleet, namespaced tool surface.** The gate fronts several MCP servers at once. A route
  names its `server` (a route is tool → server → recipe → gateArg) and is keyed by **(server, tool)**, so
  two servers may both expose `search_code` and both be routed, each to its own recipe. The gate
  advertises them apart, as `<server>__<tool>` — `github__search_code`, `local-tools__search_code` — and
  forwards a cleared call downstream under the server's own name, so the tool server never sees the
  prefix. Tools are prefixed **always**, even with one server connected: prefixing only on collision
  would rename a tool the agent already knew the moment you registered a second server. The gate never
  infers the server from a tool name, so adding a server cannot silently re-point a route you already
  wrote. Server names are therefore restricted to `[a-zA-Z0-9_-]` with no `__` (the advertised name is
  handed to a model, and the provider tool-use APIs reject anything else).
- **`http` context providers.** The `rag` and `mcp_resource` provider kinds are reserved and fail closed
  (an unbuildable provider is dropped from the session, never fabricated). Keeping retrieval in a
  *downstream* provider is deliberate: it is what lets the gate stay model-free.

## Verified interop

`stag-proxy` is a Go (`github.com/modelcontextprotocol/go-sdk`) MCP server. It has been driven
end-to-end by the **official MCP Inspector** (a TypeScript-SDK client, an independent implementation)
over stdio: `tools/list`, a cleared `tools/call` (forwarded to the real server), and a denied
`tools/call` (blocked before the downstream, the refusal surfaced to the client) — cross-implementation
compatibility, not just self-tests.
