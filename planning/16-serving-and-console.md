# 16 — Serving + the live-testing console (HTTP API, the stoa-graph console, docker-compose)

Recorded 2026-07-03. The runtime runs end-to-end from the CLI (Planning/12, U13–U21). This phase packages it
as infra and puts a browser console in front for live testing and tuning. **Decisions (Curtis): repurpose
the existing `/home/local/stoa-graph` console (Next.js 16 + React 19 + Tailwind v4); package with
docker-compose.** The console mockup already maps 1:1 onto stag concepts — it needs a decide INPUT and
real API wiring, nothing redesigned.

## The good news: the console already fits

`stoa-graph/app/page.tsx` is a polished monitoring console with the right vocabulary: allow/deny/escalate
verdict pills, a **decision stream**, a **detail** panel (verdict, subject class, sink, rule fired, a
sense→reason→decide→act→prove chain, a **signed record** with ed25519 key + leaf hash + "Verify chain"), and
a **stats** row. All mock data. Repurposing = (1) add an incident-event input to drive decisions, (2) wire
the panels to the runtime API. The design system (`globals.css`) already defines the verdict colors.

> Note: `AGENTS.md` warns this is **Next.js 16** with breaking changes — read `node_modules/next/dist/docs/`
> before writing frontend code; do not assume older Next conventions.

## The API contract (what both repos build to)

A new HTTP transport wraps `runner.Engine` (transport-agnostic since U17 — this is also the Planning/13
ingress adapter). Enforcement stays synchronous inside `Engine.Decide` (inv 9); the HTTP handler is just a
caller. The event remains untrusted Input, so a browser submitting events changes nothing about the trust
model.

```
POST /api/decide      {"event": "<incident text>"}  ->  DecideView (below)
GET  /api/log         ->  {"leaves": [LeafView...], "verify": VerifyView}
GET  /api/recipe      ->  {"name","hash","labels": [{"label","tier"}]}   (the offline `check` map)
GET  /api/health      ->  {"ok": true, "model", "embedder"}
```

`DecideView` — the current `DecideResult` **enriched** so the Detail panel can show *why* (this enrichment is
the one real backend change beyond transport):

```jsonc
{
  "verdict": "deny|allow|escalate",
  "label":   "restart_service",           // normalized label (the crossing value)
  "model":   { "raw": "…full completion…", "normalized": "restart_service", "name": "openai:mistral" },
  "retrieved": [ {"id","source","score"} ],   // the RAG docs the model saw (NEW: Engine must surface these)
  "actions": { "cleared": [ActionView], "recommend": [...], "denied": [...] },
  "subjectClass": "untrusted",            // the proposal is untrusted-until-gated
  "ruleFired": "action.auto",             // AuthorizingRule / the gate rule
  "chain":   { "sense":"ok","reason":"ok","decide":"deny","act":"skip","prove":"ok" },
  "fired":   [ {"ref","fired","output","err"} ],
  "events":  [ ReleaseEvent... ],
  "record":  { "signed": true, "keyId":"…","leaf":"sha256:…","verified": true },  // if a checkpoint exists
  "fault":   "", "egressErr": ""
}
```

Pipeline → the mockup's chain: **sense** = event received; **reason** = retrieve + assemble + model propose;
**decide** = the gate; **act** = fire-iff-cleared; **prove** = record + (rung 2) sign. A deny/escalate makes
`act = skip` and `decide = deny|escalate`.

Backend enrichment needed (small, real): the Engine currently discards the retrieved docs and the raw model
completion (the normalizer overwrites the value). `Engine.Decide` must optionally return the retrieved
`kb.Doc`s (id/source/score) and the raw completion, and expose the sink/gate outcome detail for
deny/escalate paths (today `DecideResult` only carries a `ReleaseEvent` on allow).

## Frontend work (stoa-graph)

1. **Add a decide input** — an incident-event textarea + "Decide" button (the mockup lacks an input; it is
   the live-testing entry point). A scenario picker seeded from `deploy/incident/scenarios/*` is a nice add.
2. **Wire the panels** — Decision stream ← the session's decisions (+ `GET /api/log`); Detail ← the selected
   `DecideView` (verdict, label, subject class, rule, chain, reasoning, signed record); "Verify chain" button
   ← `GET /api/log`'s verify status; Stats ← counts computed from the session.
3. **Reach the API** — Next.js `rewrites` proxy `/api/*` to the backend (avoids CORS), or the Go server sets
   permissive dev CORS. Prefer the rewrite.

## Packaging (docker-compose)

```
services:
  stag:      # Go runtime: serves /api on :8080, bundles deploy/ task assets
    volumes: [ ./state:/state ]        # events.jsonl + keys survive restarts
    environment: [ OLLAMA_URL, ... ]   # config env-overridable (portability)
  console:   # stoa-graph Next.js: `next start` on :3000, proxies /api -> stag:8080
  # ollama: host (host.docker.internal, models already pulled) OR a sibling service
```

Config portability is a prerequisite: the ollama `base_url` and the deploy paths must be env-overridable so
the container points at host ollama or a compose service instead of a hardcoded WSL IP.

## Honest notes

- **No auth, by design** — fine on localhost / an internal network; do NOT expose the console publicly without
  a token. Out of scope for the dev console.
- **Actuators stay the echo stub inside the container.** Real effects from a container is a
  privilege/blast-radius conversation, explicitly out of scope here.
- The gate is unchanged; this phase adds a transport and a UI, not policy.

## Build sequence

1. **Backend** (Go, ladder-built): enrich `DecideResult` (retrieved docs, raw completion, deny/escalate
   detail) + the HTTP transport (`/api/decide|log|recipe|health`) wrapping the Engine. Fuzz the transport
   dispatch as with `runner.Serve`.
2. **Frontend** (stoa-graph): decide input + wire the panels to the API (Next.js 16 — read its bundled docs).
3. **Packaging**: config env-overridability, Dockerfile (static Go build + assets), Next production image,
   `docker-compose.yml` (stag + console + ollama wiring + state volume).

## Status

**Backend BUILT 2026-07-03 (U23, transcripts/serve-u23-http-api.md).** The `serve` package + `stag-serve`
cmd expose `/api/decide|log|policies|health` over the gating `proxy.Gate` — fail-closed, fuzzed 2.0M,
PRODUCTION-CLEAN, live over real HTTP. Note the endpoint shape landed on the MCP-proxy gate (tool calls),
per Planning/17, not the ollama incident engine — `/api/decide` takes `{tool, args}` and returns a
`DecisionView`.

**Frontend WIRED 2026-07-03 (DEVLOG "Console wired").** The stoa-graph Next.js console drives every panel
from the live API (`app/lib/api.ts` + a client `app/page.tsx`) — a tool-call input, the Decision stream, the
Detail with the provable-loop chain, and the Signed-record panel from `/api/log`. `next build` typechecks
clean; verified end-to-end (SSR shell + cross-origin decide + CORS preflight). Direct CORS fetch
(`NEXT_PUBLIC_API_BASE`), no proxy. **Next: docker-compose** (stag-serve + console + state volume; ollama not
needed for the gating proxy). The CLI, the gate, and the egress layers are untouched.
