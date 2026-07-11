# stag

The StAG kernel: pure, deterministic algebra built test-first with the Go ratchet, gated by real
oracles and fuzzed. Kernel COMPLETE (U1-U7) and packaged (`github.com/scanset/StAG`); broker phase
underway (U8 recipe boundary built). See `../../../Planning/07-build-plan.md` and the DEVLOG.

## Packages

- `internal/trust`   — TrustClass (U1)
- `internal/gate`    — Verdict (U2) + SinkGate (U3), imports trust
- `internal/release` — ReleaseRule (U4)
- `internal/record`  — CanonicalHash (U5) + ReleaseEvent (U6), imports trust
- `stag` (root, public) — RecipeEval/`Eval` (U7, evolved to the recipe graph: branch, gate, Escalate,
  Fault, recipe_hash), composes the internals and re-exports the primitive types/constants plus the
  Parse inverses and CanonicalHash a recipe author needs. The primitives stay internal (not
  author-configurable — closed at the gate).
- `recipe` (public, broker phase) — RecipeParse (U8): the YAML recipe boundary, parser + author-time
  linter, producing a `stag.Recipe` and the two hashes. Depends on the maintained yaml fork
  `go.yaml.in/yaml/v3`, quarantined here; the kernel packages stay stdlib-only.
- `model` (public, broker phase) — Proposer (U9): the untrusted proposer strategy (Proposer interface,
  LocalStub, Decide). Confers zero trust — a Proposal has no trust field, and Decide's verdict depends
  only on the proposal value, never on the model. Imports the kernel (outer→inner); the kernel never
  imports it.
- `model/claude` (public, broker phase) — Claude adapter (U10): a `model.Proposer` calling the Anthropic
  Messages API (github.com/anthropics/anthropic-sdk-go, quarantined here). Same zero-trust boundary; fails
  closed on API errors and refusals; never sends temperature. Tested against an httptest server.
- `model/openai` (public, broker phase) — OpenAI-compatible adapter (U11): one `model.Proposer` over
  /v1/chat/completions covering ollama/vllm/OpenRouter/OpenAI by base_url+key. Hand-rolled, STDLIB-ONLY (no
  dependency). Same zero-trust boundary; fails closed; never sends temperature. httptest-tested; runs live
  against ollama for free local testing.
- `config` (public, runtime phase) — System config (U13): the StoaGraph instance's infra wiring
  (proposer/embedder/kb/egress/transport). Fail-closed YAML loader (KnownFields), pure (no env reads).
  The task layer (recipe, prompt, actuator bindings) is separate. Standalone; no kernel/model/broker dep.
- `kb` (public, runtime phase) — Knowledge base / embedding RAG (U14): Embedder (ollama /v1/embeddings,
  hand-rolled) + MemStore + LoadDir (markdown → chunk → embed) + Retrieve (cosine top-k) + Chunk/Cosine.
  A pure retriever; makes no trust/gate decision (retrieved docs are untrusted, handled at the gate).
  In-memory v1 behind a Store interface; stdlib-only.
- `bind` (public, runtime phase) — Context assembly (U15): Assemble(instruction, event, docs) → model.Request.
  Trusted instruction → System (verbatim); untrusted event + retrieved docs → Input, labeled as data. The
  trust-position invariant (System == instruction for any untrusted content) is fuzz-proven (17.4M execs).
- `broker` (public, broker phase) — Broker core (U12): the model-decision proxy's decision engine.
  Broker{Recipe, RecipeHash, Proposer, Sink}; Decide composes model.Decide, emits ReleaseEvents to the
  EventSink (async, best-effort), and shapes the result into Cleared/Recommend/Denied actions. The shaping
  law (nothing denied/escalated is ever cleared; no event dropped) is fuzz-proven vs an independent oracle.
  Stdlib + model + stag; no dependency.

## Units (dependency order)

- **TrustClass** (U1) — the trust label and the `Join` that propagates it. A meet-semilattice: taint
  spreads downward, a class is only ever lowered by combination. *Built, fuzzed, harden-clean.*
- **Verdict** (U2) — the gate output `{Allow, Escalate, Deny}` and the AND/OR/negate rollup. *Built, fuzzed, harden-clean.*
- **SinkGate** (U3) — value class + sink sensitivity + release-applied -> Verdict (Caller is deny-unless-released). *Built, fuzzed, harden-clean.*
- **ReleaseRule** (U4) — closed-set release predicate (the declassifier's mechanism; canonical-only numeric). *Built, fuzzed, harden-clean.*
- **CanonicalHash** (U5) — deterministic sha256 over sorted-key JSON. *Built, fuzzed, harden-clean.*
- **ReleaseEvent** (U6) — the record of a trust-crossing and its hash. *Built, fuzzed, harden-clean.*
- **RecipeEval** (U7) — in-memory wiring of U1-U6; its fuzz target is the whole-product safety invariant.
  *Built, fuzzed (27.8M execs), adversarially verified, harden-clean.*

## Build

Since U3 the units are authored by hand (Claude) and gated by the ratchet's deterministic oracle tools,
not the local-model generate flow. From `harness/`, per unit:

- `bash tools/spec_check.sh` (specs on stdin as `=== name.spec ===`) — validate the spec pair.
- `bash tools/tdd_red.sh stag` (test on stdin) — RED: the test compiles against the stub AND fails.
- `bash tools/stage_impl.sh stag` (impl on stdin) — GREEN: `go vet` + `go test -race`, writes impl only.
- `bash tools/go_fuzz.sh stag` — run every `FuzzXxx` briefly; then a longer `go test -fuzz`.
- `bash tools/go_quality.sh stag` — the full harden gate (gofmt/vet/build/test -race/staticcheck/govulncheck).

The `ratchet flow . tdd --ws stag` generate ladder still exists but is off the critical path (it was slow/
flaky on this hardware). Specs for composed units live in `specs/_built/`; `specs/` holds the in-flight unit.
