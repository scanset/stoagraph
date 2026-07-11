# 27 — Nailing down the MCP gate for v1 (spec)

Recorded 2026-07-06. Companion to Planning/17 (mcp gating proxy), /24 (stag-proxy), /26 (release readiness).
Scope: make `stag-proxy` — the MCP gating proxy — **release-solid** for the "MCP gate first" v1 (Planning/26).
This is robustness + a clear contract, NOT new features.

## The unit

Take the MCP gating proxy from "works in our own tests" to "an outside adopter can point a standard MCP client at
it and trust it." Concretely: verified cross-implementation interop, the rough edges a standing daemon exposes
closed or documented, an integration contract doc, and the v1 boundaries stated (not silent).

## What the MCP piece IS (the contract to preserve)

`stag-proxy` is an MCP **server** to the agent and an MCP **client** to the downstream, with the deterministic
`proxy.Gate` in the middle. Every `tools/call` is gated; **forward-iff-cleared**; a denied/escalated call returns
an MCP tool error (`isError:true` + `_meta.stag`) and never reaches the downstream. No model, no keys in the
enforcement path. Two modes: **stdio** (single agent, global route table) and the **daemon** (session→recipe over
streamable HTTP). v1 gates the **tools/ACT channel only**.

## Gaps → plan → status

1. **No real-client verification** (only our go-sdk test client). → Drive the official MCP Inspector (a
   TypeScript-SDK client) over stdio: `tools/list`, a cleared call, a denied call. — **DONE**: Inspector listed
   the gated tools; `get_pods(dev)` allowed+forwarded (real pods); `delete_namespace(staging)` denied, refusal
   surfaced, staging survived.
2. **Daemon sessions never expire** (`NewStreamableHTTPHandler` got nil opts). → Set
   `StreamableHTTPOptions.SessionTimeout` (30 min idle). — **DONE** (build+test green).
3. **No downstream reconnect** — if the downstream MCP server dies, the shared session breaks and all sessions
   fail. → v1: DOCUMENT as a supervised-process limitation (run under a supervisor that restarts it); auto-reconnect
   is v1.1. — **documented** (contract §Scope + deferred list).
4. **`decodeArgs` stringifies non-scalar args** via `fmt.Sprint`. → v1: gated args are scalars (compared as
   strings); DOCUMENT the boundary. — **documented** (contract §Scope).
5. **Tools-only** (no resources/prompts). → v1 boundary (context READ channel = v1.1); the server advertises only
   the tools it serves (SDK registers only tools). DOCUMENT it. — **confirmed + documented**.
6. **No integration contract.** → Write `release/docs/mcp-gating-proxy.md`: what it is, the guarantee, stdio vs
   daemon, the refusal shape, v1 scope, verified interop. — **DONE** (drafted, staged).

## Done criteria (release-solid)

- [x] A real, independent MCP client drives `tools/list` + a cleared + a denied call (cross-implementation).
- [x] The standing daemon does not leak protocol sessions (idle timeout).
- [x] An accurate integration contract doc exists in the release staging area.
- [x] The v1 boundaries (tools-only, scalar gated args, single downstream, unauthenticated control plane,
      supervised downstream) are stated, not silent.
- [x] `go build ./... && go vet ./... && go test ./...` green after the changes (16 packages pass).

## Out of scope (v1.1 — Planning/26)

Context READ channel (gated resources), multi-downstream, control-plane auth, downstream auto-reconnect,
per-token Registry TTL (the token→recipe map is ephemeral / cleared on restart; bounded by dispatch rate between
restarts in v1).

## Open decisions (none block v1)

- Registry token TTL strategy (lazy-expire vs sweep vs single-use) — v1.1.
- Whether the daemon's control plane ships with a built-in bearer-token option or stays "put it behind your own
  auth" — v1.1 auth work.
