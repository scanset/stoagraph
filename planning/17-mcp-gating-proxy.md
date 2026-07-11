# 17 — stag as a gating MCP proxy (the tool boundary, realized)

Recorded 2026-07-03. **Decision (Curtis): stag becomes a gating MCP proxy** — it exposes itself as an
MCP server that agents connect to, and gates every tool call before it reaches the real (downstream) MCP
servers. Plus: **recipes must be authorable and bind to the right context providers and MCP tools.** This is
not a side feature; it is the original concept (00/01: "the boundary where an autonomous AI agent acts on the
world — a tool call") realized literally. It reframes the whole system, so this doc fixes the architecture
before the build.

## The flip: the agent proposes, stag gates — no model in the enforcement path

The incident use case (U13–U21) was the **Tier-3 model-decision proxy**: stag ran its OWN model (ollama)
to propose a label. The MCP gating proxy is different and simpler: the **external agent** proposes the action
(an MCP `tools/call` with a name + arguments), and stag gates it **deterministically** — no LLM of its
own in the enforcement path. The agent brings the (untrusted) intelligence; stag is the deterministic
control. This is *more* on-thesis, not less: the gate's whole value is being deterministic over an untrusted
proposal, and here the untrusted proposal arrives ready-made.

```
   agent (untrusted)                 stag                          real MCP servers
   ─────────────────                 ─────────                          ────────────────
   tools/call ──────▶  MCP SERVER ─▶ map call → recipe ─▶ Eval (gate)
   {name, args}         (faces         (deterministic; no model)
                         the agent)          │
                                    cleared  │  denied / escalate
                                       ▼      └────────▶ MCP error / hand to human
                          MCP CLIENT ─────── forward ───────▶  tools/call ─▶ result
                          (faces real servers)                                  │
                                       ◀──────── result (UNTRUSTED) ─────────────┘
                                       │
                          record + sign (egress rungs 1–2), return result to the agent
```

stag is **both** an MCP server (to the agent) and an MCP client (to the real servers), with the gate in
the middle. The tool *result* flowing back is untrusted — if the agent feeds it into its next reasoning, the
agent's next tool call is gated the same way. Treating tool results as untrusted is exactly what naive agent
loops get wrong and what makes the proxy valuable.

## This maps onto the kernel we already have

An MCP tool call is a structured proposal: a tool name + typed arguments. That is precisely the kernel's
**ingredients** (arg slots, each carrying an origin + trust class — Context Binding) feeding the **graph**
(branch/gate/sink). The incident use case only exercised a single `propose → out` slot; the tool-call case
exercises the multi-slot ingredient path the kernel was built for. So:

- **Reused unchanged:** the kernel (`Eval`, trust classes, gate, release rules, recipe graph + parser),
  broker shaping (cleared/recommend/denied), egress (hash chain + signed checkpoints), config, canonical hash.
- **The proposer becomes a pass-through:** instead of `model.Proposer` calling ollama, the proposal is the
  agent's tool call. The model-decision proxy and the gating proxy are two front-ends over one kernel.
- **New:** the MCP server side, the MCP client side, the tool-call → recipe router, the arg → ingredient
  binder, the `ContextProvider` interface, recipe authoring bound to providers/MCPs, and the console.

## Tool call → recipe (routing + fail-closed default)

A recipe declares which tool(s) it governs (by name or pattern). An incoming `tools/call`:
1. routes to its recipe (a tool with no matching recipe → **fail-closed default: deny**, do not forward a
   tool stag has no policy for);
2. binds each argument to a recipe ingredient with the recipe's **declared trust label** for that arg
   (invariant 14 — soundness given the labels the operator declares, not their inferred truth; stag does
   not guess provenance, the recipe author states it);
3. `Eval` runs the graph → allow/deny/escalate;
4. cleared → the MCP client forwards the call downstream and returns the result; else → an MCP error (deny)
   or an escalation hand-off; always → record + sign.

## Recipes that bind context providers + MCPs (authoring)

The second requirement: author recipes that "work using the right context providers and MCPs." A recipe in
the proxy world binds three things declaratively:
- **the MCP tool(s) it governs** (name/pattern → the downstream server + tool);
- **the trust label of each argument** (Context Binding — the arg's origin);
- **the rule data it needs from context providers** — e.g. a `deploy_service` recipe whose release rule
  checks an allowed-images set from a CMDB provider, or a change-window provider that gates by time. In the
  deterministic proxy, a **context provider** supplies *rule inputs and escalation context*, not a model
  prompt (there is no model in the path). It is the same untrusted-data abstraction as RAG (generalizing
  `kb.Retriever`), pointed at rules instead of a prompt.

Authoring surface = the console's **Policies** tab; provider/MCP wiring = the **Adapters** tab; live gated
calls = **Live**; signed records = **Records**. The mockup's navigation is the product surface.

## Dependencies (quarantined)

The MCP Go SDK is a third-party dependency and the first that is load-bearing in the *enforcement path*
(server + client). It is **quarantined** in its own package(s) (like `model/claude`); the kernel/broker/egress
stay stdlib-clean. The gate never imports MCP; the MCP layer calls the gate.

## Honest scope — this is a program, not a unit

This is materially larger than the serving/console phase. Build it as a **walking skeleton first**, then
broaden:

**Slice 0 (walking skeleton, the proof):** stag runs an MCP server exposing ONE proxied tool from ONE
downstream MCP server; a client calls it; a hand-written recipe gates the call (allow one arg value, deny
another); cleared → forward to the real server and return the result; denied → MCP error; the decision is
recorded + signed. End-to-end proof of the gating proxy with everything reused.

Then, in order: (1) the tool-call → recipe router + arg binder over the real recipe format; (2) the
`ContextProvider` interface + one real provider (RAG generalized, then an MCP-resource provider); (3) recipe
authoring API + the Policies/Adapters console tabs; (4) multiple downstream servers, escalation hand-off,
richer arg-trust binding; (5) the HTTP API + console wiring (Planning/16) as the operator surface over all of
it.

## Honest ceilings / trust notes

- **Provenance is declared, not inferred.** stag gates on the trust labels the recipe author assigns to
  a tool's arguments. A wrong label is a policy bug (inv 14). This is the honest ceiling of the whole system,
  now at the tool boundary.
- **Tool results are untrusted** and are not re-injected into any stag model (there is none); the agent's
  handling of them is the agent's risk, but its next call is re-gated.
- **Unknown tool → deny** (fail closed): stag never forwards a tool it has no recipe for.
- **The gate stays synchronous** (inv 9): forwarding happens only after a cleared verdict, in-path; recording
  and signing are off-path.
- Auth between the agent and stag (who may connect) is a real concern for the proxy — deferred with the
  console's auth (internal/trusted transport first).

## Refinement (2026-07-03, Curtis): proxy BOTH channels — tools AND context

stag is not just an MCP *tool* proxy; it is an **MCP + context-provider proxy**. The agent is configured
to reach stag for *everything* it reads and does — so nothing bypasses the broker. That is the point:
**complete mediation of the agent's entire external I/O**, not just its actions.

An agent has two external verbs, and stag proxies both:

```
                          ┌──────────────  stag  ──────────────┐
   agent ── READ  ───────▶│ context proxy: label untrusted + record │──▶ real context providers
        (context)         │   (+ optional allow/deny per policy)    │     (MCP resources, RAG, HTTP, DB…)
        ── ACT   ────────▶│ tool proxy:    the deterministic gate   │──▶ real MCP tool servers
        (tool call)       │   allow / deny / escalate               │
                          └─────────────────────────────────────────┘
                       every read and every act is recorded in the signed log
```

- **ACT (tools) → gated.** Allow / deny / escalate, forward iff cleared (Slice 0). The write boundary.
- **READ (context) → mediated.** Every context fetch is stamped **untrusted** at origin and **recorded**;
  a policy may additionally allow/deny a provider or filter results. The read boundary — new as a first-class
  proxy target.

**Why forcing reads through stag matters (this is the deeper half):**
- **Injection becomes unbypassable to label.** The #1 agent attack is context injection (poisoned RAG,
  adversarial MCP resources, malicious tool *results*). If ALL context flows through stag, every piece is
  labeled untrusted at the source — the agent can never receive "trusted-looking" context that skipped
  labeling. Tool results are context too: they re-enter through the read proxy as untrusted and re-gate the
  next act. This is invariant 3 (trust by origin) made **complete** — origin is stamped at the one mandatory
  chokepoint.
- **Total provenance.** Every input and output of the agent is in the signed log (U19/U21). You can replay
  exactly what the agent saw and did.
- **One control point.** Recipes govern the act side; provider policy governs the read side; both live in one
  broker the agent cannot route around.

**In MCP terms** a full MCP proxy already spans both verbs: **tools** (act, gated) and **resources/prompts**
(read, mediated). Non-MCP context (RAG, HTTP, a CMDB) is proxied via **context-provider adapters** behind the
same read boundary. So the `ContextProvider` abstraction (Planning/16, and the broadening list below) is not
just "a source that feeds a prompt" — it is **a proxied read channel** stag sits in front of.

**Honest asymmetry.** Gating *reads* is usually lighter than gating *acts*: the read proxy's core job is
**label-at-origin + record** (and optionally allow/deny a provider), not primarily to deny data. But making
reads flow through stag is exactly what makes the untrusted-labeling and the audit **unbypassable** — the
value is the mandatory chokepoint, not heavy read-time denial.

**Effect on the plan.** The admin console's **Adapters** section configures both MCP tool servers *and*
context providers, and stag proxies both. The context-provider proxy is a first-class capability
alongside the MCP tool gate — a broadening slice, not an afterthought. The recipe-authoring slice (in
progress) is unchanged: recipes are the act-side policy.

## Status

Architecture fixed. Supersedes the framing of Planning/11's "Tier 3 proxy" as the *only* front-end — the
gating MCP proxy is a second front-end over the same kernel, and the strategic one. Planning/16 (serving +
console) becomes the operator surface over this.

**Slice 0 BUILT 2026-07-03 (U22, transcripts/proxy-u22-mcp-gating-slice0.md).** The walking skeleton works
end-to-end over real MCP transports: agent → stag gating server → gate → stag client → downstream;
a denied call never reaches the downstream tool. Core `proxy` package (forward-iff-cleared, fuzzed 4.2M) +
quarantined `proxy/mcpgate` adapter (MCP SDK v1.6.1, go 1.25). The kernel/recipe/egress were reused unchanged.
**Next: broaden** — (1) tool→recipe router + multi-arg→ingredient binding over the real recipe format;
(2) the `ContextProvider` interface + an MCP-resource provider; (3) recipe authoring + the Policies/Adapters
console tabs; (4) a runnable `stag-mcp-proxy` over stdio for a real MCP client; then fold into Planning/16.
