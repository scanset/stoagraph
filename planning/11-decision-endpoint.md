# 11 — The decision endpoint (the model-decision proxy)

Recorded 2026-07-02, ratified by Curtis the same day. The first broker front-end is the **synchronous
decision endpoint** (Planning/04 Tier 3, "proxy the model too / full loop"): StAG runs the agent's
model turn through a proposer, evaluates the recipe graph, and returns the gated decision. Transport:
in-process Go API first, then stdio JSON-RPC; HTTP and a real MCP gateway come later behind the same
core. This doc fixes the architecture, the proposer-tier strategy, and the build ladder.

## What the endpoint is

The kernel decides; `model.Decide` composes a proposer with `Eval`; this endpoint is a thin
transport shell + policy loader + egress sink around that core. Enforcement is synchronous (the caller
blocks on the verdict); egress is asynchronous (the signed leaf leaves after). One request in, one
gated decision out.

Flow for one decision:

```
request (bound event/context)
  -> Broker: run the injected Proposer (LocalStub | Claude | OpenAI-compatible) -> Proposal (Untrusted)
  -> stag.Eval(recipe, proposal.Value, semanticHash) -> EvalResult
  -> emit each ReleaseEvent to the EventSink (async egress)
  -> shape and return Decision:
       Verdict, ClearedActions (Allowed authoritative sinks), Recommendations (Escalate), Denials, Events
```

The proposal is Untrusted by construction (it is the model's output), so there is no tool/arg-trust
question at this boundary - that is the MCP tool-gateway's concern (Tier 1, deferred). Complete
mediation still holds structurally: every authoritative sink runs the crossing gate; nothing crosses
at Allow without a recorded, hash-bound ReleaseEvent (the U7 v2 invariant, unchanged).

## The Broker core

A `broker` package (import github.com/scanset/StAG/broker), depending on model + recipe + the stag
root. It holds the decision as configuration and adds egress + response shaping over `model.Decide`:

- `Broker{ Recipe stag.Recipe; RecipeHash string; Proposer model.Proposer; Sink EventSink }`.
- `Decide(ctx, Request) (Decision, error)`: runs `model.Decide`, writes each emitted ReleaseEvent to
  the Sink (best-effort, non-blocking on the verdict), and returns the shaped Decision.
- `EventSink` interface: `Record(ctx, ReleaseEvent) error` (+ each event's Hash). In-memory default
  (observable in tests); a JSONL writer and the ProofLayer/Rekor connector plug in later. Per the
  recon (Planning/08), ProofLayer has no external-hash ingestion path today - real egress stays
  deferred; the interface is the seam.
- `Decision` DTO: Verdict; ClearedActions (SinkOutcomes that are Allow at an authoritative sink);
  Recommendations (the Escalate items - the recommend-only path, refuse-with-plan in v1); Denials;
  Events (with hashes); Fault; plus the proposal provenance (never authorizing).

The security-bearing logic is transport-agnostic and deterministic with `LocalStub`, so it goes
through the full oracle ladder.

## Proposer-tier strategy (resolves the ollama / vllm / OpenRouter questions)

Two adapters cover every backend:

- **Claude** (U10, done) - the one bespoke SDK adapter (github.com/anthropics/anthropic-sdk-go,
  quarantined in model/claude).
- **OpenAI-compatible** (`model/openai`, to build) - a single adapter over the OpenAI
  `/v1/chat/completions` shape, selected by base_url + key. It covers **ollama** (local, free -
  ollama serves an OpenAI-compatible endpoint at :11434/v1), **vllm**, **OpenRouter**, and **OpenAI**.
  "The OpenRouter piece" is this adapter pointed at OpenRouter; no separate adapter.

Decision (ratified): do NOT pivot ollama -> vllm now. Keep ollama for local dev iteration; the
OpenAI-compatible adapter makes the whole loop testable locally for zero tokens, and the same adapter
reaches OpenRouter/OpenAI later by config. Adopt vllm only for throughput or logprobs; adopt LiteLLM
(unified cross-backend spend/telemetry) only when operational model metrics are a concrete goal.

Telemetry stance: the assurance telemetry (verdict -> cleared-actions -> ReleaseEvent action-chain) is
broker-emitted via the EventSink and is the point of "actionable observability" - it does not depend
on the model server. Model-serving telemetry (latency, throughput, GPU) is operational and deferred.
Optionally surface per-call model usage (tokens, model, latency) as extra trust-free provenance on the
Proposal when the record layer wants it; not built speculatively.

## Transport (ratified: in-process, then stdio)

- **In-process** (v1): the Broker is a Go library called directly; this is what the ladder tests and
  what a Go host embeds.
- **stdio JSON-RPC** (v1): a thin loop over stdin/stdout exposing a `decide` method, so a non-Go agent
  can drive the broker locally. Tested against in-process pipes - no network.
- **Later, behind the same core**: an HTTP decision endpoint (stdlib net/http) for network deployment,
  and a real MCP gateway (Tier 1, tool-call gating) when that boundary is wanted.

## v1 honest ceilings

- Escalate = refuse-with-recommendation (return the plan + a requires-approval marker); there is no
  live human-approval channel or single-use token mint yet (ZT-reference roadmap).
- Egress is the EventSink interface with an in-memory default; the signed, append-only, externally
  anchored leaf is the next layer (ProofLayer/Rekor), deferred.
- One recipe per Broker instance in v1; multi-recipe routing / per-session recipe selection is later.
- The Broker runs one proposer per instance (injected); model choice is the integrator's (Planning/10).

## Build ladder

Each a full ladder run (spec_check, red, green, fuzz where an invariant exists, quality); the core is
security-semantic and gets an adversarial pass.

1. **broker core** - Broker + EventSink + Decision, over model.Decide, tested with LocalStub
   (deterministic). Fuzz: the shaping law (ClearedActions are exactly the Allow authoritative-sink
   outcomes; every Event is bound to RecipeHash and hashes cleanly; Recommendations iff Escalate;
   Denials otherwise). Adversarial: can the shaped Decision ever present a Denied/Escalated crossing as
   cleared, or drop an Event? Fail-closed on proposer error (Deny).
2. **model/openai adapter** - httptest-tested (like Claude); runs live against ollama's /v1. Same
   zero-trust boundary; fails closed on API errors; never sends temperature.
3. **stdio JSON-RPC transport** - wraps the Broker; tested against in-process pipes. Fail-closed decode.
4. **live local run** - stdio broker + OpenAI-compatible adapter on ollama, end to end, free; then
   Claude (env.local) / OpenRouter; then HTTP / MCP.
