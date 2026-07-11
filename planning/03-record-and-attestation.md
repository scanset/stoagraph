# 03 — Record and Attestation

The record is the byproduct of enforcement, not a separate pipeline. The same gate that authorizes
an action produces its leaf, so the evidence cannot be skipped or forged after the fact. This doc
covers the native leaf StAG produces, the hashing discipline it carries from ESP, the two things it
adds beyond ESP, and the egress connectors that carry the leaf to a verifiable log.

## The three-layer leaf (carried from ESP)

ESP's manifest is the right shape and it is verified in code
(`Endpoint-State-Policy/execution_engine/src/types/canonical_manifest.rs`). Each gated action
becomes a leaf with three layers:

- **Intent.** What was supposed to happen: the rule that could fire, the expected shape, the
  declared action set. Reconstructable without the runtime values.
- **Contract.** The provenance of acquisition: how each input was obtained. In ESP this carries
  `collector_id` and `collection_mode` (`canonical_manifest.rs:695-708`). StAG adds
  `trust_class` here, because a trust class is the identical category of fact (provenance of
  acquisition), and it hashes for free.
- **Outcome.** What actually happened: the verdict and the per-field results.

Leaves roll up through a CRI-tree with AND / OR / negate combinators
(`canonical_manifest.rs:318-329`), exactly as ESP composes criteria. A multi-step action is a
hash-linked chain of these leaves; the completed chain's head is what gets appended to the log.

## The hashing discipline (carried verbatim)

This is the part to copy exactly, because it is the part that is easy to get subtly wrong. ESP's
determinism is structural:

- Every hash-feeding map is a `BTreeMap`, never a `HashMap`.
- Every hash-feeding vec is sorted before hashing. ESP sorts child hashes with the comment "Sort
  for determinism (AND/OR are commutative)" (`canonical_manifest.rs:349-355`). They understood
  *why*, not just that it works.

In Go there is no `BTreeMap` and map iteration is randomized, so this discipline is rebuilt by
hand: sort keys explicitly, fix struct field order, format numbers stably, and canonicalize before
hashing. The operational rule: **authors write YAML, StAG canonicalizes to JSON, and only the
canonical JSON is hashed.** Nothing that a human edits is ever the thing that gets signed.

## Intent-only reconstruction (carried from ESP)

ESP can reconstruct the intent manifest from the policy and the asset without re-running collectors
(`execution_engine/src/execution/engine.rs:301-328`), yielding a byte-identical hash input for
drift detection. StAG carries this: given a recipe and its declared inputs, it can recompute the
intent and contract layers and confirm the signed leaf matches, after the fact, without re-running
the agent.

## What StAG adds beyond ESP

ESP models trust-*avoidance*: it compares a collected value to a static expectation and discards
the value, so nothing ever crosses a boundary. StAG must attest trust-*crossing*. Two additions,
both anticipated by ESP's own code:

1. **A verdict enum instead of a bool.** ESP's per-object outcome is `passed: bool`
   (`CtnObjectHash`, `canonical_manifest.rs:476-507`), and its own doc comment anticipates widening
   to a tri-state "without breaking the hash primitive, the hash already excludes outcome
   metadata." StAG widens it to `{Allow, Deny, Escalate}`. The hash primitive is unaffected.

2. **The ReleaseEvent as a first-class hashed layer.** There is no home in ESP's outcome model for
   "input X of class `untrusted` was released to authoritative field Y under rule R, by actor A,"
   because ESP records comparison results, not release decisions. StAG adds it as a new hashed
   layer that sits natively alongside the per-criterion record, in the same idiom (BTreeMap-
   canonical, hash-stable, reconstruction-friendly). Sketch:

   ```
   ReleaseEvent {
     subject_class:     "untrusted"          // what crossed
     subject_origin:    "retriever.runbooks" // from where
     collected_field:   "classify.output.action"
     target_class:      "authoritative"      // to where
     target_field:      "act.args.action"
     authorizing_rule:  "actions.approved"   // under which rule
     actor:             "policy:remediation" // by whose authority
     ordering:          7                    // when (sequence position)
   }
   ```

   Hashed independently and folded into the leaf, so the authorization decision is recorded at the
   same granularity as the verdict, and it flows through the same deterministic rollup.

## Descriptive plus normative

The run record answers *what happened* (the inputs, the render, the tool output). The verdict and
the ReleaseEvent answer *why it was authorized* (the rule that fired, the release that was
permitted). StAG's leaf carries both, which is what makes it evidence rather than a log line: an
auditor can reconstruct not only the effect on the world but the decision that allowed it.

## Egress: the verifiable log is consumed, not built

StAG produces the signed leaf. It does not build the append-only transparency log. That is a
consumed component, reached over a connector:

- **ProofLayer** already implements an RFC 6962 Merkle transparency log with inclusion and
  consistency proofs and P-256-signed checkpoints (`crates/prooflayer-2/src/transparency/`), and it
  ingests signed results at `POST /api/scans`. StAG's leaf can be shaped to that ingest payload and
  pushed over a webhook, so the leaf inherits inclusion, consistency, and non-equivocation without
  StAG building any of it. This is the natural integration, and it closes the loop on the reference
  architecture's stubbed verifiable log: the thing the ZT doc consumes is built next door.
- **Rekor / RFC 6962 directly**, or an internal transparency log, are alternate connectors for a
  customer not running ProofLayer.
- **OTel / SIEM** are observational egress: necessary, but the authoritative record is the signed
  leaf in the verifiable log, not the trace.

The egress is asynchronous and does not gate. The gate already happened, synchronously, before the
action. Egress is how the evidence leaves.

## Control mapping (downstream, not in StAG)

Mapping a verdict to a compliance control (EU AI Act, ISO 42001, NIST) happens in the downstream
system, not in StAG. ProofLayer stores control mappings supplied at ingest and serves them by
framework and control id (`crates/prooflayer-2/src/evidence/`), but it has no auto-mapping registry
and no AI-framework content yet. StAG's job is to emit a verdict leaf clean enough that a mapping
layer can consume it. Building the AI-framework control content is a separate effort on the
ProofLayer side, listed here only so the seam is explicit.
