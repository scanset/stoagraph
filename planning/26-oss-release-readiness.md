# 26 — StoaGraph open-source release readiness (ALL of it — unified)

**DECISION REVERSED 2026-07-11 (Curtis): the WHOLE product goes open source — there is no carve-out.**
`stag` (the gate) **and** `event_harness` (the orchestrator) **and** both consoles ship together as one
open-source product, packaged into Docker. The earlier plan (v1 of this doc, 2026-07-06) open-sourced only
`stag` and kept the orchestrator private; that framing is dead. Everything below is rewritten against the new
decision. **Restructuring waits until we package** — nothing moves in the repo until then (see "The move,
when we're ready").

## What this changes (vs the old carve-out plan)

| Old plan (dead) | Now |
| --- | --- |
| Ship `stag` only; `event_harness` stays private | **Ship everything** — one repo, one product |
| Fresh repo w/ clean history so a security product doesn't carry commercial strategy | No strategy to hide. History hygiene is now only about **secrets** (keys, config.db, jsonl) — already gitignored |
| Examples must run WITHOUT the orchestrator | The orchestrator ships, so **the turnkey k8s demo IS the demo** (event → dispatch → gated agent → approval) |
| Two consoles, one public (stag) one private (StoaGraph) | **One console** — merge them (below) |
| Scrub comments naming the "commercial event_harness" | Moot. `stag` / `event_harness` / `StoaGraph` stay as **component names**, not a license boundary |
| `Planning/` + `DEVLOG` stay private (they describe the split) | Optional — they no longer leak strategy. DECIDE whether to ship or trim |

The **architecture rationale is unchanged and still sells itself**: the gate holds no model and no keys; the
orchestrator holds them. That was never a licensing boundary — it's a *security* boundary, and it stands on
its own merit with the source open.

## The two things "together" means

### 1. One console (merge the frontends)
Today:
- **`stoa-graph/`** — Next.js, `:3000`, talks to **stag-serve `:8080`** (`NEXT_PUBLIC_API_BASE`).
  Pages: `/` · `/recipes` · `/adapters` · `/approvals`. Covers recipes, routes, mcp-servers, providers, approvals.
- **`event_harness/cmd/harness-serve/index.html`** — a single embedded HTML file served by **harness-serve
  `:8090`**. Covers models, event map, dispatch + the live SSE transcript.

Target: **the Next.js app absorbs the orchestrator surfaces.** Add `/models`, `/event-map`, `/dispatch`
(with the SSE transcript view) to `stoa-graph`, and retire the embedded `index.html`. The console then talks to
**both** services — policy/approvals from stag-serve, models/dispatch/SSE from harness-serve — via two
configurable bases (`NEXT_PUBLIC_STAG_API` + `NEXT_PUBLIC_HARNESS_API`). Keep the two BACKENDS separate (the
gate must stay independently runnable — that's the whole point); only the UI unifies. Brand it **StoaGraph**
(the whole product); `stag` remains the name of the gate inside it.

### 2. One repo, one deployable
- Bring `event_harness` + `stag` under one tree with a clean top-level layout.
- **Docker** is the release artifact: `docker compose up` brings the whole product — stag-serve, stag-proxy
  (daemon), harness-serve, the console, plus the example downstreams (k8s-ops MCP server, kbserve).
- Keep them as **separate services/containers** (not one binary): the gate's independence is the product claim,
  and the compose file *demonstrates* the topology rather than hiding it.

## The move, when we're ready (NOT yet)

Recon done 2026-07-11; execute at packaging time:
- `harness/workspaces/stag` → **`stag/`** at repo root. The module is **fully portable** — self-contained
  `go.mod` (`github.com/scanset/StAG`), builds standalone, **no `replace` pointing back into `harness/`**.
- **The one load-bearing link:** `event_harness/go.mod` has
  `replace github.com/scanset/StAG => ../harness/workspaces/stag` → must become `=> ../stag`.
  (Go *imports* use the module path, so only this replace cares about the filesystem location.)
- Note 4 untracked files live under stag — use a plain `mv` of the directory (carries tracked **and**
  untracked), then `git add -A`; git detects the renames.
- **Delete `harness/`** — the vendored Ratchet authoring harness (flows, kb, `.index`, `tools/`, `runs/`;
  ~702 tracked files). It is **dev tooling only** — nothing in stag or event_harness builds or runs against it.
  Ratchet is its own repo.
- Then fix the stale path references: `deploy/README.md`, `k8s_test/README.md` (build commands),
  the DEVLOG operating notes at the top, `icm/ratchet/*` + `icm/workflows/*` (they document the Ratchet loop
  that's going away). Leave **historical DEVLOG entries alone** — they're history, not instructions.

Proposed final layout:
```
stag/          the gate (Go module: kernel, proxy, serve)
event_harness/ the orchestrator (Go module: dispatch, agent, models, kbserve)
console/       ONE Next.js app (was stoa-graph/ + the embedded index.html)
examples/      k8s (turnkey demo), pii-demo
deploy/        docker-compose, Dockerfiles, helm
docs/          public docs
```

## Checklist

### A. License + identity
- **Pick a LICENSE** — **Apache-2.0 recommended** (patent grant + permissive; the right default for
  security/infra). DECIDE.
- **Module path / GitHub org + repo name** (e.g. `github.com/stoagraph/stoagraph`) — moving off
  `github.com/scanset/StAG` rewrites imports across both modules. DECIDE.

### B. Restructure (the move above) + build green
- `go build/vet/test ./...` green in **both** modules at the new paths; console `npm ci && build` green.

### C. Console merge
- Port models / event-map / dispatch(SSE) into the Next.js app; retire the embedded `index.html`;
  two configurable API bases; StoaGraph branding.

### D. Docker / deploy
- Dockerfile per service + `docker compose up` for the whole product (incl. example downstreams).
- Optional: helm chart to deploy StoaGraph itself (distinct from the k8s *demo app's* chart).

### E. Docs (public, written fresh)
- **README:** the thesis — deterministic gate, **no model in the enforcement path**, complete mediation on
  BOTH channels (ACT: allow/deny · READ: label+record), signed audit — plus a 5-minute quickstart and the
  architecture diagram. Now it can show the FULL loop (event → dispatch → gated agent → approval).
- **Recipe authoring guide** — grammar, rules, node kinds, linter, and the load-bearing invariant.
- **SECURITY.md / threat model** — what the gate protects, what it does NOT, fail-closed posture, and the
  **trust invariant** (the READ label is positional, not taint-propagation; the ACT gate re-derives trust at the
  sink — see Planning/30). Credibility-critical.
- **Deployment/topology** — now the whole topology, orchestrator included.

### F. CI + hygiene
- GH Actions: gofmt, `go vet`, `go test ./...` (both modules), console build.
- Secrets sweep: no `config.db`, `*.key`, `*.jsonl`, `models.json`, `.env.local` (all gitignored — verify).
- v0.1.0 tag + CHANGELOG.

## Open decisions (need Curtis)
1. **License** — Apache-2.0 (recommended) vs MIT.
2. **Org + repo name + module path** (import rewrite).
3. **Do `Planning/` + `DEVLOG.md` + `icm/` ship?** They're no longer strategy-sensitive, but they're a lot of
   internal history. (Recommend: ship a trimmed `docs/`, keep Planning/DEVLOG in the repo but out of the
   published docs site.)
4. **Single Docker image vs compose of services** (recommend: compose — it *shows* the gate's independence).

## Resolved since v1 of this doc
- ~~Context-provider READ channel deferred to v1.1~~ → **BUILT** (Planning/30): context is served by the gate as
  untrusted MCP resources, recorded. Both channels now cross the gate.
- ~~Open/commercial line~~ → **there isn't one.**

## Still deferred (document as "coming", don't ship half-built)
- **Multi-downstream** (one gate fronting several MCP servers).
- **Control-plane auth** (bearer token on POST /sessions, approvals-approve, admin CRUD; today dev-open).
  **This one matters more now** — an all-OSS product will get deployed by strangers.
- **Tier-2 OAuth downstream auth** (Planning/28), `mcp_resource` context providers (Planning/30).
