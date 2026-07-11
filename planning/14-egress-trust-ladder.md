# 14 — The egress trust ladder (and why PKI is a later feature)

Recorded 2026-07-03. The broker already emits `ReleaseEvent`s through the `EventSink` seam; today the
only sink is in-memory. This doc fixes what the egress layer must become and, crucially, **sequences
the cryptography by threat model** so we build the useful part now and defer signing/PKI until the
threat model demands it. **Decision: v1 egress is a hash-chained JSONL sink with NO keys and NO PKI.
The signed-head and external-anchor layers (and any full PKI) are cached as later features.**

## The seam is already there

`broker.EventSink` is a one-method interface (`Record(ctx, ev) error`). The ladder below is a stack of
implementations behind that seam — each wraps the previous, none changes the broker:

```
broker ──ReleaseEvent──▶ EventSink
                          ├─ MemSink            (have it; demo only)
                          ├─ JSONLSink          (v1 egress — THIS is next; no PKI)
                          ├─ SigningSink        (wraps JSONL; one keypair; DEFERRED)
                          └─ TransparencyConnector (ProofLayer/Rekor; DEFERRED)
```

Because the layers compose, building the hash chain now commits us to nothing about keys.

## The ladder: property → mechanism → cost → what it defends against

| Rung | Property | Mechanism | PKI? | Defends against |
| --- | --- | --- | --- | --- |
| 1 | **Tamper-evidence** — the log wasn't edited since you last saw its head | hash chain: each record carries `prev_hash`; a running head hash. Reuses the sha256 canonical hashing already in the stack | **No** | partial edits, insertions, deletions — detectable by anyone holding a trusted head |
| 2 | **Authenticity / non-repudiation** — this log came from THIS deployment | sign the head with ONE asymmetric keypair (Ed25519 / P-256); distribute the public key out-of-band | **One keypair, not a CA** | wholesale rewrite by a party lacking the private key |
| 3 | **Operator-proof append-only** — even we can't silently rewrite the past | publish signed heads to an external witness / transparency log | **Yes, but delegated** | a compromised stag operator rolling back history |
| 4 | **Ambient identity** — the signer is tied to an OIDC/workload identity, not a static key | Sigstore Fulcio (short-lived certs) + Rekor | **Full X.509 PKI** | key-management burden; "which key was this, and who held it" |

## The threat-model hinge (why rung 1 is enough to start)

- Adversary = an **external tamperer who does not control the log store** → rung 1 (hash chain) is
  sufficient. The chain proves the sequence is intact relative to a trusted head.
- Adversary = a **compromised stag deployment** (the interesting case, since the pitch is "don't
  take the operator's word, here's proof") → rungs 2–3 are needed, because whoever controls the store
  can rewrite the chain AND its head. That is precisely the work being deferred.

**Honest limit of rung 1, stated up front:** hash-chain-only gives tamper-evidence *relative to a
trusted head*. It does NOT stop a total rewrite by someone who controls the store and whose head you
have no independent copy of. Closing that gap is exactly what rungs 2–3 buy — and the right place for
them is the transparency connector, not reinvented in stag.

## Why the PKI lives in the connector, not in stag

The reuse map already assigns signing to a sibling: ProofLayer provides "RFC 6962 log and **P-256
signing** for the verifiable-log layer." So the intended architecture **delegates** rungs 2–4 to the
transparency connector (ProofLayer, or Rekor). stag produces hash-chained records and hands heads
off; it never stands up a CA, and at most holds a single signing key that can itself be pushed to the
connector. Full X.509 / certificate-authority PKI enters only at rung 4 (Sigstore-style ambient
identity), which is a product decision, not a prerequisite.

## What we build now vs. later

- **NOW (v1 egress, next build unit):** `JSONLSink` — append-only newline-delimited records, each
  carrying its canonical hash and the prior record's hash (`prev_hash`), plus a stable head. Pure,
  fail-closed, no keys. Tested with the same fuzz-against-an-oracle discipline: the chain-integrity
  invariant (any reordering / edit / drop breaks verification; a clean append verifies).
- **LATER (deferred, this doc):**
  - **Rung 2 — `SigningSink`:** one Ed25519/P-256 keypair signs the head; public key distributed
    out-of-band. Minimal-PKI, no CA.
  - **Rung 3 — `TransparencyConnector`:** submit signed heads to ProofLayer's RFC 6962 log (or Rekor);
    external witnesses make rollback detectable.
  - **Rung 4 — ambient identity (Sigstore Fulcio):** only if a customer wants OIDC/workload-bound
    signers instead of a static key. Full PKI; product-driven, not assumed.

## Status

Explained and sequenced (this doc). Build order: `JSONLSink` (hash chain, no PKI) next; signing and
transparency anchoring deferred to the ProofLayer/Rekor connector; full PKI (rung 4) only on demand.
The `EventSink` seam means each rung is an independent, wrapping implementation — deferring the crypto
costs no rework. Pairs with Planning/13 (ingress): both ingress and egress crypto sit OUTSIDE the
synchronous gate.
