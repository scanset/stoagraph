# StAG Planning

**StAG (Structural Assurance Graph).** A deterministic assurance broker for autonomous AI
agents. Structural assurance for event-driven agentic AI, layered on a Zero Trust architecture.

This directory holds the envisioning and planning docs. Nothing here is built yet. The build
target is a Go product: a broker that sits at the tool boundary of a customer's existing agent,
computes an origin-bound trust label, gates each proposed action against a deterministic policy,
declassifies deliberately and legibly when an untrusted value must reach an authoritative field,
and records the whole thing as a signed, verifiable leaf.

StAG is the implementable core of the reference architecture in
`/home/local/Ratchet/docs/preview/ZT-Reference.md`. The reference doc describes the pattern;
StAG is the extractable kernel of it, pointed at someone else's agent.

## Naming (canonical, settled 2026-07-06)

- **`stag` / `StAG` (Structural Assurance Graph)** — the OPEN-SOURCE gate: the kernel, the MCP gating
  proxy (`stag-proxy`), `stag-serve`, and the authoring console. `StAG` is the formal acronym / module
  path (`github.com/scanset/StAG`); `stag` is the lowercase brand + binary names. Ships to `StAG-Release`.
- **`event_harness`** — the COMMERCIAL orchestrator (models, dispatcher, approval retry loop). Its
  console is branded "StoaGraph".
- **`StoaGraph`** — the whole PRODUCT = `stag` + `event_harness`.

Many docs below predate this split and use "StoaGraph" to mean the gate — **read those as `stag`.**

## The through-line

An AI agent is an untrusted, non-deterministic proposer. Safety does not come from making the
model trustworthy. It comes from bracketing it: controlling what enters its context (origin-bound
context binding) and placing a deterministic gate downstream of the planner that authorizes or
refuses each proposed action. The gate's authorization is reproducible even though the planner is
not. Everything else (taint propagation, the gate, the record, control mapping) is buildable or
already built next door. The declassifier is the one hard, unsolved center, and building a
legible, kernel-closed one is the reason StAG exists.

## The governing law

**Open at the edges, closed at the gate.** The developer integrates only at a typed,
trust-classed boundary (a tool declares its sink sensitivity, a retriever declares its output
class, a model is an untrusted-until-gated proposer). The kernel (taint propagation, the gate,
the declassifier) is not pluggable and not author-configurable. You bring your agent; you do not
bring your own gate.

## Documents

- [00-concept.md](00-concept.md) — what StAG is, the extracted-kernel framing, scope, and the
  non-goals that keep it tight.
- [01-architecture.md](01-architecture.md) — the binder → gate → declassifier → record pipeline,
  the broker shape, and the Go implications.
- [02-declassifier.md](02-declassifier.md) — the hard center: the four dimensions, the five
  strengths, the closed-set rule, policy-as-data, and the honest ceiling.
- [03-record-and-attestation.md](03-record-and-attestation.md) — the native three-layer leaf, the
  hashing discipline carried from ESP, the verdict enum, the ReleaseEvent, and the egress
  connectors.
- [04-adapter-surface.md](04-adapter-surface.md) — the recipe schema, the adapter contract, and
  the three integration tiers for bringing an existing agent (for example LangChain).
- [05-open-problems.md](05-open-problems.md) — the honest open problems, in priority order.
- [06-positioning.md](06-positioning.md) — the marketing-heavy messaging house: category, pillars,
  audience, objection handling, and landing-page raw material (with honesty guardrails for the
  page). Deliberately a different register from the technical docs.
- [07-build-plan.md](07-build-plan.md) — the technical build plan: the vendored Go ratchet harness,
  the kernel decomposition into seven TDD units (bottom-up), the fuzz targets, and the build
  sequence. This is where envisioning turns into code.
- [08-recipe-format-recon.md](08-recipe-format-recon.md) — broker-phase recon: the recipe format
  verdict (lint-first shape plus grafts), the YAML threat surface verified against the yaml.v3
  source, the 18 fail-closed parser rules, the 9 format laws, and the ratified decisions.
- [09-use-case-and-graph.md](09-use-case-and-graph.md) — the first use case (the ZT reference
  scenario as one recipe) and the ratified graph semantics: path-following Eval, branch, gate
  checkpoints, the structural source of Escalate, and the kernel build order they force.
- [10-gate-configuration-boundary.md](10-gate-configuration-boundary.md) — resolves "the gate is
  never author-configurable" against the fact that recipes exist: the three layers (mechanism and
  vocabulary locked, parameters authored), why the authored layer is monotonically safe, the
  select-an-oracle-never-supply-one trap, and the label-honesty ceiling. Diagrams.
- [11-decision-endpoint.md](11-decision-endpoint.md) — the first broker front-end: the synchronous
  model-decision proxy (Tier 3). The Broker core over Decide, the EventSink egress seam, the
  proposer-tier strategy (Claude bespoke + one OpenAI-compatible adapter covering
  ollama/vllm/OpenRouter/OpenAI), in-process→stdio transport, and the build ladder.
- [12-runtime-and-usecase.md](12-runtime-and-usecase.md) — the stag runtime and the first use case:
  the context trust model (trusted instruction + untrusted event/RAG as data), the infra
  incident-remediation label-selection scenario with its recipe, the injection-resistance sweep,
  embedding RAG (ollama nomic-embed), and the runtime components (config, kb, bind, actuator, stdio).
- [13-event-ingress.md](13-event-ingress.md) — the event-listener seam: where a production event source
  belongs (outside the gate; events are untrusted Input; invariant 9 async-in/synchronous-gate/async-out),
  in-process vs out-of-process wiring, and the two deferred listeners (message-queue consumer leaning
  primary, HTTP webhook receiver second) — each a quarantined adapter in front of the decide seam.
- [14-egress-trust-ladder.md](14-egress-trust-ladder.md) — the egress ladder behind the EventSink seam:
  hash-chained JSONL (v1, no PKI) → signed head (one keypair) → external transparency anchor
  (ProofLayer/Rekor) → ambient identity (Sigstore, full PKI). Sequences the crypto by threat model;
  signing/PKI deferred to the connector, so v1 needs no keys.
- [15-signed-egress.md](15-signed-egress.md) — rung 2 spec-out: self-signed Ed25519 **checkpoints** over the
  chain head (`Checkpoint{origin,count,head}` + sig), stdlib-only, no new dependency; key management
  (keygen, gitignored private key, `signing` config block); what it closes (outsider forgery) vs. its honest
  ceiling (operator rollback needs the rung-3 external witness). Previews the ProofLayer/Rekor connector.
  BUILT (U21).
- [16-serving-and-console.md](16-serving-and-console.md) — packaging + the live-testing console: an HTTP API
  wrapping the transport-agnostic Engine (`/api/decide|log|recipe|health`, also the Planning/13 ingress
  adapter), the enriched `DecideView` contract, repurposing the `stoa-graph` Next.js console (add an input,
  wire the panels), and a docker-compose topology (stag + console + ollama, state volume). Backend first.
- [17-mcp-gating-proxy.md](17-mcp-gating-proxy.md) — the strategic reframe: stag as a **gating MCP
  proxy** (MCP server to the agent + MCP client to real servers, the gate in the middle). The agent proposes
  tool calls; stag gates deterministically (no model in the enforcement path) — the kernel's
  ingredients+trust path, reused unchanged. Tool→recipe routing (unknown→deny), recipes that bind MCP tools +
  context providers (authoring in the console's Policies/Adapters tabs), the quarantined MCP SDK, and a
  walking-skeleton Slice 0 to prove it. A program, phased. **Refinement:** proxy BOTH channels — tools (act,
  gated) AND context providers (read, labeled+recorded) — "forces everything through stag."
- [18-adapters.md](18-adapters.md) — the Adapters slice: MCP-server adapters (act, gated) + context-provider
  adapters (read, untrusted+recorded) behind one interface each; the **SQLite config store** (pure-Go
  modernc; recipes stay text; the relational win is the bindings — route/provider_binding/recipe_dep); wiring
  the live gate to the store (multi-tool router); the Adapters console tab. Build sequence + honest scope.
- [19-recipe-model-extensions.md](19-recipe-model-extensions.md) — deferred recipe-model features:
  **foreach** (bounded fan-out — RESERVED-and-rejected in the parser today; the construct that lets the model
  propose a runtime list / plan and gates each element — "AI as an intelligence source") and **composition**
  (a sub-recipe as an alternative action path; recommended compile-time inlining, invariant-preserving).
- [20-operational-model-and-console-ux.md](20-operational-model-and-console-ux.md) — considerations for later
  (Curtis + Gemini): the dual-proxy operational model (LangChain sync → MCP gate out; webhooks/events async →
  taint-at-ingest in), the non-bypassable-boundary truth, and two console-UX ideas — the **Taint Map**
  (data-flow trust audit; the DENY "gotcha") and the **Reactor** (async event → propose → dispose).

## Reuse map (patterns, not runtime dependencies)

StAG is a standalone Go product. It reuses *patterns* from three sibling engines, and integrates
with one of them as an egress connector. It does not depend on any of them at runtime.

| Source | What StAG borrows | Relationship |
| --- | --- | --- |
| Ratchet (Go) | the engine shape: origin-aware binding, the gate, the run record, the author-time linter | pattern reuse; StAG is the kernel extracted and pointed outward |
| ESP (Rust) | the attestation discipline: three-layer intent/contract/outcome leaf, CRI-tree rollup, deterministic hashing, intent-only reconstruction | pattern reuse, re-implemented natively in Go |
| ProofLayer (Rust) | nothing at runtime | egress connector (webhook/API): consume its RFC 6962 log and P-256 signing for the verifiable-log layer |

## Status

Kernel phase complete: U1-U7 built through the ratchet oracles, fuzzed, adversarially verified,
packaged (`harness/workspaces/stag`, module `github.com/scanset/StAG`). Broker phase open: recipe
format chosen and decisions ratified (08), first use case and graph semantics set (09). Next: the
kernel evolution ladder (enum register, recipe_hash, graph Eval), then the recipe parser and
linter. Current state always lives in the top entry of `../DEVLOG.md`.
