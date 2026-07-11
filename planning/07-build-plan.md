# 07 — Build Plan (kernel, TDD-first)

This is the technical plan for the first build phase: the StAG kernel algebra, built test-first with
the Go ratchet, gated by real oracles, and fuzzed. No broker, no IO, no adapters yet. Just the pure
core the whole product rests on, proven unit by unit.

## The harness

The Linux/go ratchet is vendored into this repo at `harness/`. The product source is a workspace
under it at `harness/workspaces/stag` (`module stag`, go 1.23). The ratchet travels with the repo so
the build is reproducible: the harness that builds StAG is pinned alongside it.

Invocations (run from `harness/`):

- Draft or revise a spec: `ratchet flow . spec --ws stag "<description>"`
- Build a module from specs (compile + test): `ratchet flow . compose --ws stag ""`
- The TDD ladder (the one we want): `ratchet flow . tdd --ws stag`
- Full gate over the workspace: `ratchet flow . harden --ws stag`

First run builds the KB search index; if a flow reports no index, run the ratchet's index step once.

The `tdd` flow is the assurance ladder, and it is the reason we are building here rather than by
hand: read specs, stub (oracle: compiles), red test plus a property/fuzz target (oracle `tdd_red`:
compiles and fails against the stub, so a trivial test is rejected), green impl (oracle: `go vet`
plus `go test -race`), fuzz (oracle `go_fuzz`: native `FuzzXxx` finds no crash), harden (`go_quality`:
gofmt, vet, build, test -race, staticcheck, govulncheck, gosec). Every repair is a feedback cycle
fed the oracle's exact output.

## Module strategy: flat now, repackaged later

The ratchet composes one flat `package main` module (every unit is a file at the module root, no
cross-imports). That is a constraint of the compose model, not the product's final shape. We accept
it deliberately for the kernel phase, because the kernel is pure algebra that fits a single package
cleanly, and it keeps `go test ./...` covering the whole thing. When the kernel is proven, it gets
lifted into real packages (`github.com/scanset/StAG` with `internal/trust`, `internal/gate`,
`internal/release`, `internal/record`) as the first step of the broker phase. The specs and tests
carry over; the package split is a mechanical move once the behavior is locked.

## The kernel, bottom-up

Seven units, in dependency order. Each is one component `.spec` plus one test `.spec` carrying a
`FuzzXxx` target. All pure, deterministic, standard-library only.

### Layer 0 — algebra (no dependencies)

- **U1 TrustClass.** The trust label that rides every value, and the `Join` that propagates it. A
  meet-semilattice: `Join(a,b)` is the least-trusted of the two, so taint spreads downward and a
  value's class is only ever lowered by combination, never raised. Raising a class is declassification
  (U4), a separate deliberate act. *This is the IFC label. It is unit one because everything binds to
  it.*
- **U2 Verdict.** The gate's output enum `{Allow, Deny, Escalate}`, plus the rollup combinators
  (AND / OR / negate) that compose child verdicts into a parent, the CRI-tree shape carried from ESP.
  *This is both the gate output and the attestation rollup.*

### Layer 1 — decisions (depend on Layer 0)

- **U3 Sink gate.** Given a value's `TrustClass`, a sink's sensitivity (`authoritative` | `benign`),
  and whether a release applies, return a `Verdict`. An untrusted value at an authoritative sink is
  denied unless a release cleared it; a benign sink is recorded but not release-gated. *This is the
  ABAC decision at the sink.*
- **U4 Release rule (the declassifier's mechanism).** A closed-set release predicate: membership in a
  declared enumerable set, equality against a signed value, a bounded numeric range. Deliberately
  narrower than a general policy language: no free computation, no content inspection. Returns whether
  a specific value may be released. *This is the hard center's evaluation step, and its fuzz target is
  the laundering test.*

### Layer 2 — records (depend on Layers 0 and 1)

- **U5 Canonical hash.** Deterministic sha256 over canonical (sorted-key, fixed-order) JSON. The Go
  restatement of ESP's `BTreeMap`-plus-sort discipline. Properties: stable (same logical content in
  any map order hashes identically) and sensitive (any field change changes the hash).
- **U6 ReleaseEvent.** The record of a crossing (`subject_class`, `subject_origin`, `collected_field`,
  `target_class`, `target_field`, `authorizing_rule`, `actor`, `ordering`) and its hash via U5. *The
  object that attests trust-crossing, which ESP never had to represent.*

### Layer 3 — composition

- **U7 Recipe eval (in-memory).** Wire U1 through U6 over a tiny bind graph: bind ingredients
  (label via U1), take an opaque proposal, gate a sink (U3), route a crossing through the release
  rule (U4) emitting a ReleaseEvent (U6), roll up the verdict (U2). No broker, no network. *This is
  the integration proof that the kernel composes.*

## Fuzz targets (the adversarial spine)

Fuzzing is not decoration here; it is where the safety claims are tested. Per unit:

- **U1:** fold `Join` over a random class sequence, assert it equals the minimum, and that `Join`
  never returns a class greater than either input.
- **U2:** rollup commutativity and associativity; `negate` is an involution.
- **U4:** throw crafted values at a closed-set rule; assert release happens only for true members and
  no crafted non-member is ever released. This is laundering resistance at the rule level.
- **U5:** hash stability under map reordering, and sensitivity to any single-field change.
- **U7 (the load-bearing one):** drive an `untrusted` value toward an `authoritative` sink under
  random recipe shapes, and assert the invariant the whole product rests on: **no untrusted value
  reaches an authoritative sink without both a gate verdict and a recorded ReleaseEvent.** If this
  fuzz target ever finds a path, that is a product-defining bug, not a test failure.

## What is deliberately deferred

- The broker (synchronous decision endpoint, MCP proxy, gRPC/HTTP).
- The recipe YAML parser and the author-time linter (the runtime kernel comes first; the linter proves
  properties over the same graph the kernel evaluates, so it is easier once U7 exists).
- Adapters and egress connectors.
- The real Go package split (the flat module is repackaged after the kernel is proven).

## Sequence

1. U1 TrustClass, via `tdd`. Establish the pattern: component spec, red test with a fuzz target,
   green, fuzz, harden.
2. U2 Verdict.
3. U3, U4 (the gate and the declassifier's rule).
4. U5, U6 (the record and the crossing).
5. U7 (composition), whose fuzz target is the safety invariant.
6. Harden the whole module, then lift into real packages to open the broker phase.

The first spec pair (U1) is written into `harness/workspaces/stag/specs`. Review it as the pattern;
the rest follow the same shape once the shape is approved.
