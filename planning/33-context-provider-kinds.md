# 33 — Context provider kinds: static, skill, and the signature tier

Recorded 2026-07-12. Planning/30 built the READ channel (gate-served resources, label + record,
never deny) with one provider kind: `http`. This doc ratifies the full kind taxonomy — what gets
built, what gets buried, and in what order — plus two READ-channel hardening items that apply to
every kind. Companion to /30 (the channel), /28 (downstream auth, reused by `mcp_resource`),
/32 (ingress; converges with resource subscriptions), /15 (signing machinery, reused by `skill`).

## The taxonomy (ratified)

| Kind | Status | What it is | Outbound query? |
| --- | --- | --- | --- |
| `http` | built | GET a downstream endpoint with `?q=`; the universal adapter | yes — bounded (below) |
| `static` | **build next (C1)** | content-addressed local bundle, served verbatim | **no** |
| `skill` | build (C3) | a `static` bundle with selection-by-name + a **signature tier** | **no** |
| `mcp_resource` | build (C4) | proxy a downstream MCP server's `resources/read` (Planning/28 transport) | yes — bounded |
| `rag` | **buried — doctrine** | gate-side embedding retrieval | n/a |

## `rag` is buried, not reserved: the gate never embeds

"Reserved" implies coming. It is not coming. **Doctrine: the gate never embeds** — an embedder is
a model, and a model in the gate forfeits the determinism claim the product stands on. Retrieval
is a *downstream* concern: `kbserve` (markdown chunk + embed + cosine behind `http`) is the
reference pattern, and any vector store an operator likes can sit behind the same kind. The
`rag` config value stays permanently rejected by `provider.FromConfig` (fail closed, logged), and
the docs stop listing it as a kind.

## `static` — the content-addressed bundle (C1)

The kind the first real deployments actually need: runbooks and reference docs, no moving parts.

- **Register:** `POST /api/providers {name, kind:"static", config:{path}}` — a file or directory.
  At registration the gate walks it, canonicalizes (sorted paths, LF), and computes a bundle hash
  `sha256(manifest)` plus per-file hashes. The hash set is stored with the provider row.
- **Serve:** `resources/read stag://context/<name>` returns the bundle (or a named file within it)
  verbatim, stamped untrusted as always. **There is no `?q`** — no retrieval, no filtering, no
  outbound anything. Whole-document beats similarity search at runbook scale, and the absence of a
  query removes an entire egress class (below).
- **Evidence:** every read record carries the bundle hash — the audit says *the model saw exactly
  these bytes*. A re-registered (edited) bundle gets a new hash; the old hash stays meaningful in
  old records.
- **Bound:** size cap at registration (a bundle that cannot fit a context window is a config
  error, not a runtime surprise).

## `skill` — a procedure, not a fact pile (C3)

A **skill** is a curated, versioned, content-addressed bundle that teaches the model *how to do
something* (a procedure), as opposed to context that tells it *what is true* (facts). Mechanically
it is `static` plus two additions:

**1. Selection semantics.** Skills are chosen deliberately, never similarity-retrieved: bound per
definition (`skills:["triage-k8s"]` beside `tools` and `context` in the event map) or read by name
(`stag://skill/<name>`). A skill is loaded because the *operator* decided this workflow uses this
procedure — the same authorship posture as a recipe.

**2. The signature tier.** The one thing generic context does not have:

| Tier | Verification | Trust position |
| --- | --- | --- |
| **Signed skill** | ed25519 signature over the bundle hash, verified by the **harness** against the operator's public key | eligible for the **instruction slot** (System) |
| **Unsigned skill** | none | **Input slot** — untrusted reference, like any context |

This formalizes what `deploy/incident/instruction.md` already is (an operator-authored trusted
instruction), using the signing machinery that already exists (Planning/15 keys; approvals). It
does **not** violate Planning/30's never-trust-a-flag doctrine: the gate serves the skill, its
hash, and its signature; the **harness verifies the signature itself** before System placement.
Trust comes from cryptography the reader checks, never from a label the channel asserts. A skill
whose signature fails verification degrades to unsigned (Input slot) and the failure is recorded.

**Why this kind earns its keep.** The worst case for installing an arbitrary third-party skill is
a lying document in the data slot: it can inform, it cannot instruct (positional), and it cannot
act (every proposal still hits the gate). That is the ClawHub failure mode — malicious skills at
marketplace scale — converted into a non-event by construction. And every session records
`{skill, version, hash, tier}`, so the audit answers a question nobody else's record can: *which
procedure informed this decision?*

## `mcp_resource` — ride the ecosystem (C4)

Proxy a downstream MCP server's `resources/read` through the gate: discover its resources at
registration (the Planning/28 transport + auth, unchanged), bind them as `stag://context/<name>`,
stamp + record on read like everything else. Build after `static` proves the multi-kind plumbing.
One deliberate future hook: MCP resource **subscriptions** (`resources/subscribe`) make a
downstream update a push notification — which is an *event*, and lands in Planning/32's ingress
map as an attributed source (`source: "mcp:<server>"`). The READ channel and the ingress plan
converge here; do not build subscriptions before /32's I2 exists to receive them.

## Hardening (applies to every kind)

1. **Chain the read log.** `reads.jsonl` is a plain append log while the ACT decisions log is
   hash-chained. READ crossings are evidence — *what the model saw* — and deserve the same
   discipline: hash-chain the read log and include the per-item content hash in each record.
   (With `static`/`skill` this is what makes "the model saw exactly these bytes" checkable.)
2. **Bound the outbound query.** Reads are never denied, but `?q` is agent-influenced text flowing
   *out* to the provider's endpoint — the canary-exfiltration class, READ-side. The endpoint is
   operator-chosen, which bounds *where*; nothing yet bounds *what*. Per-binding query policy:
   `query: "verbatim" | "bounded" | "none"` (bounded = length cap + charset allowlist; default
   **bounded**), recorded per read. `static` and `skill` are `none` by construction — a further
   reason they should carry most production context.

## Build ladder

| Unit | Builds | Pass bar |
| --- | --- | --- |
| **C1** | `static` kind: register (hash at registration), serve, size cap | a directory of markdown serves through the gate with no kbserve; read record carries the bundle hash |
| **C2** | read-log chaining + per-item content hashes; query policy on `http` | reads verify like decisions; an over-long `q` is truncated and the truncation recorded |
| **C3** | `skill` kind: selection by name, event-map `skills:[]`, signature tier, harness-side verification | a signed skill lands in System; the same skill with a broken signature lands in Input with the failure recorded |
| **C4** | `mcp_resource`: downstream resource proxy (discover, bind, stamp, record) | a registry MCP server's resource reads through the gate, labeled + recorded |
| **C5** | console: provider health + read audit surfaced in Adapters | a failing provider is visible (fail-open no longer means invisible); reads browsable per session |

C5 answers the other half of "the context provider feels lacking": today a failed provider
silently yields empty context (correct — read-fail-open — but invisible). Correct and invisible
is how a gap *feels* like a flaw.

## The honest ceiling

- A signed skill is trusted **authorship**, not verified **correctness** — signing proves the
  operator published it, not that the procedure is right. A wrong signed skill steers the model
  inside the instruction slot; the gate still bounds every action it inspires. Say this plainly
  in the docs.
- The untrusted stamp remains positional (Planning/30). Nothing in this doc adds taint-tracking
  through the model, and nothing may claim it.
- Query bounding shrinks the READ-side exfiltration channel; it does not close it. `none`-query
  kinds close it, which is why the recommendation is that they carry most production context.

## Open decisions

1. Skill signing key: reuse the egress checkpoint keypair (fewest keys) or a dedicated
   skill-authoring key (cleaner revocation)? Leaning dedicated.
2. Skill versioning surface: hash-only, or a human `version:` field in a skill manifest
   (hash stays authoritative either way)?
3. `static` refresh: re-register manually (leaning — edits should be deliberate) or watch the
   path for changes?
4. Bounded-query defaults: length cap value and charset (proposal: 256 chars,
   `[A-Za-z0-9 .,:_-]`).
