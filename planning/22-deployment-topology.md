# 22 — Deployment topology: stag is the gate, the harness is the orchestrator

Recorded 2026-07-04. This is the architectural decision that split the codebase into two modules. It
supersedes the original "stag runs the whole loop" runtime (U9–U21).

## The principle

**stag owns the gate, not the agent.** The agent is untrusted, model-driven, non-deterministic. The gate
is trusted, deterministic, model-free, auditable. The moment stag runs the model or drives the loop, it
blurs the trust boundary that is its entire value. So stag is a **passive MCP + context proxy**; the model
and the loop live outside it.

## The three roles (stag is only one)

1. **Trigger** — an event happens (ticket, webhook, cron). The customer's event source. Not stag.
2. **Agent runtime / orchestrator** — reads the event, runs the model loop, proposes actions. `event_harness`
   (or any MCP-capable agent host). Holds the model keys.
3. **Gate** — mediates every tool call and context read: allow / deny / record. **stag.**

## Why MCP makes this clean

An MCP server is **passive** — it only responds; it never reaches out. So stag never listens for events,
never dispatches. The agent (however it was triggered) *connects to* stag and *pulls* its governed tools;
stag gates each proposed call and forwards the cleared ones downstream. The entire event/dispatch problem
stays on the far side of the wire, where it already lives. "Forces everything through stag" is
**topological**: the agent host is configured with stag as its sole MCP server.

## The model never sees the recipe

Structural assurance means the guarantee holds even if the agent is ignorant or adversarial. **The model has a
flashlight (the local node — tools/context reachable now); stag has the map (the recipe graph).** The
model proposes a step; stag decides if that edge is legal. You cannot jailbreak a graph you cannot
perceive. The recipe is a contract between the harness and stag — the harness selects it per event; the
**session** carries it; stag enforces it; the model just acts.

> **stag decides what is _allowed_. The harness decides what is _attempted_.** Referee, not player.

## Recipe binding: session → recipe (the "event tagged with a recipe")

The harness picks the recipe per event by **which stag session/endpoint the agent connects to** (e.g.
`/mcp/support` vs `/mcp/refunds`, or a session tag). Zero orchestration in stag — it enforces the session's
recipe on every crossing. This is the next build (`stag-proxy` daemon): `NewGatingServer` already gates + forwards;
it needs a session → recipe map.

- **Per-call gating (built):** each call gated independently by a recipe.
- **Per-session traversal (future, stateful):** the session tracks position in one recipe graph; each call is a
  legal transition; `branch`/`foreach`/`exit` become the run's control flow. This is what makes "walk the whole
  recipe" literal — a deliberate stateful build on top of the session binding.

## The split (what lives where)

**stag (`harness/workspaces/stag`, module `github.com/scanset/StAG`) — the passive gate/proxy + console:**
kernel (`stag` + `internal`), `recipe`/`recipestore`, `store` (routes, MCP-server + context-provider
associations), `proxy` + `proxy/mcpgate` (the MCP gate/forward), `provider` (context proxy: label-at-origin +
mediation), `router`, `serve`, `egress`. Binary `stag-serve` (console) + future `stag-proxy` (gating daemon).
**No LLM SDK** — the anthropic dependency left the module entirely.

**`event_harness` (module `github.com/scanset/event-harness`) — the orchestrator:** `model` + `model/claude` +
`model/openai` (model calling), `kb` (RAG retrieval), `bind` (context binding / trust-position assembly). It
runs the model, holds the keys, and (next) speaks MCP to stag as a client. It depends on the stag
kernel via a `replace` directive (shared trust types).

## Context binding — the one subtle relocation

`bind` (trust-position: trusted→System, untrusted→Input) builds the *model's prompt*, which is the orchestrator's
job. So it moved to `event_harness`. But the guarantee splits honestly:

- **Label-at-origin** (every context item read through stag is stamped untrusted + recorded) stays in
  stag (`provider.Gather`). This is the READ-channel differentiator.
- **Trust-position assembly** is orchestrator-side, kept as stag-authored reference code in the harness
  (`bind`). stag labels; the harness honors the labels when it assembles the prompt.

Trade: pure proxy in front of any agent (model-free core) vs. stag physically controlling prompt
positioning. The *property* survives; *who enforces positioning* moved. A future refinement can strengthen the
enforcement (e.g. stag delivering pre-structured, labeled context resources).

## What's next

1. `stag-proxy` daemon — expose `NewGatingServer` as a standing MCP server + session → recipe binding.
2. `event_harness` agent loop — MCP client to stag + the model loop (event → propose → gated crossing).
3. Later: per-session stateful traversal (the recipe as a session state machine).
