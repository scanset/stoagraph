# 23 — Orchestration roadmap: what the pivot defers, and why mediation comes first

Recorded 2026-07-04. Companion to Planning/22 (topology). That doc says *where the line is*; this one says *what
we traded, why the sequencing is right, and what the orchestration layer becomes.*

## The honest trade

Making stag a pure passive gate **removes the "full agent proxy" differentiator** — the version where
stag listened for events, dispatched agents, ran the model loop, assembled context, and fired actuators
(the U9–U21 runtime). That was turnkey and demo-friendly: "point your events at stag, it runs governed
agents." We gave it up on purpose.

**We did not delete that value — we re-homed it.** The runtime moved to `event_harness`. The orchestration
capability returns there, as a *separable* layer, without contaminating the trusted gate.

## Why mediation first (the load-bearing reason)

Mediation is the hard, security-critical, provably-correct part. Orchestration is valuable but forgiving —
a bug there wastes a turn; a bug in mediation lets PII cross. So the sequencing is not arbitrary:

- **Get mediation provably right on its own** — deterministic gate, no model in the enforcement path, complete
  mediation, hash-chained/signed audit, the recipe model. A small, model-free, auditable core you can trust.
- **Then build orchestration *on top of* a correct mediation core.** Orchestration on a proven gate is safe to
  iterate; orchestration entangled with the gate makes the gate un-auditable and the whole thing a monolith.

Separating them is what lets each evolve at its own pace and risk level. The gate can stay frozen-correct while
the orchestrator experiments.

## The orchestration functionality worth building (in `event_harness`, not stag)

Curtis flagged these as solid, and they are — they're the "full agent proxy" value, rebuilt cleanly:

1. **Event queue listener (ingress).** A trigger — webhook / queue subscriber / cron — that turns "an event
   happened" into "dispatch a governed agent." The customer's existing event source feeds it.
2. **Calling agents into recipes (session → recipe dispatch).** The listener maps an event *type* to a recipe
   and connects the dispatched agent to stag at the session bound to that recipe (Planning/22's
   "event tagged with a recipe"). This is the redirection layer — the orchestrator's job.
3. **The agent loop.** Model proposes → stag gates over MCP → cleared calls forward, denied ones bounce →
   results feed back. `model` + `bind` (already moved) are the pieces; the MCP-client loop is the new code.
4. **Model-provider config + keys.** Owned by the orchestrator now (removed from stag). The connectivity
   test and propose-then-gate surfaces we prototyped belong here.

## The shape of the win

Two clean layers, each strong on its own:

| | Mediation (stag) | Orchestration (event_harness) |
| --- | --- | --- |
| Trust | trusted, deterministic, model-free | untrusted, model-driven |
| Job | gate every crossing per the session's recipe; record | listen, pick recipe, dispatch agent, run the loop |
| Evolves | slowly, proven-correct | freely, iteratively |
| Value | structural assurance | turnkey "governed agents" (the re-homed differentiator) |

The "full agent proxy" appeal (plug-and-play, event → governed answer) comes back as the orchestrator matures —
but now it rides on a mediation core we can actually stand behind. **We didn't shrink the vision; we ordered
it so the load-bearing half is trustworthy before the convenient half is built on it.**

## Sequencing

1. **Mediation, finished:** `stag-proxy` daemon (standing gating MCP server + session → recipe binding). This
   completes the passive-gate story — an agent host can actually connect.
2. **Orchestration, MVP:** `event_harness` agent loop (MCP client → model loop) + model config/keys.
3. **Orchestration, full:** event queue listener + event → recipe dispatch (#1, #2 above). Designed in
   Planning/25 (the event map, the dispatch model role, the session model).
4. **Depth (either side, as needed):** per-session stateful recipe traversal; stronger trust-position
   enforcement; a local-RAG context provider.
