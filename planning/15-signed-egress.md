# 15 — Signed egress: the checkpoint (rung 2) and the transparency connector (rung 3)

Recorded 2026-07-03. Rung 1 (U19) gives tamper-evidence **relative to a trusted head**: `Verify` proves
the JSONL chain is internally consistent and returns its head, but a party who controls the store can
rewrite the whole chain and recompute a consistent head. This doc specs the rungs that close that gap —
**rung 2: sign the head** (authenticity; no external service) and **rung 3: anchor the head to an external
transparency log** (rollback-resistance) — and fixes the concrete design for rung 2 so it can be built next.
**Recommendation: build rung 2 (self-signed Ed25519 checkpoints, stdlib-only) as the next unit; it is the
artifact rung 3 submits, so it is a prerequisite, not a detour.** Rung 3 (ProofLayer/Rekor) follows.

## Where rung 2 sits

Rung 1 already produces a running **head** — the hash of the last leaf (`VerifyResult.Head`, `sink.Head()`).
Rung 2 signs a small statement about that head:

```
  rung 1 (U19)                 rung 2 (this unit)                 rung 3 (deferred)
  ────────────                 ──────────────────                 ─────────────────
  append leaves  ──head──▶  Checkpoint{origin,count,head}  ──▶  submit signed checkpoint to
  (hash chain)              signed with the deployment's         ProofLayer/Rekor; external
                            Ed25519 key  → a .checkpoint          witnesses remember it, so a
                            file beside the log                   later rollback is detectable
```

A **checkpoint** is the note-style statement `(origin, count, head)`; a **signed checkpoint** is that plus
an Ed25519 signature over its canonical bytes and a short key fingerprint. It seals a segment of the log:
anyone holding the public key can confirm the log they have is exactly what this deployment attested to.

## What rung 2 closes — and what it does NOT (honest ceiling)

- **Closes:** an outsider with no private key **cannot forge a log or a checkpoint**. A verifier holding the
  public key (distributed out-of-band) knows the checkpoint came from the key-holder and that the chain head
  matches it. The proof is **authenticated and offline-verifiable** — no network, no service.
- **Does NOT close:** the **key-holder itself** (a compromised stag) rewriting its own history and
  re-signing a shorter, consistent chain. Stopping operator rollback needs an **external witness that
  remembers a previous, larger checkpoint** — that is rung 3. Rung 2 is authenticity, not
  append-only-against-the-operator.

State this ceiling plainly wherever rung 2 ships: "signed by the deployment" is a real, useful claim (it is
what most audit trails never have), but it is not yet "the deployment cannot lie about its own past."

## Rung 2 design (concrete; stdlib-only, no new dependency)

Types and operations, added to the existing `egress` package:

```go
type Checkpoint struct {            // the statement about the head
    Origin string  // a stable label for this log, e.g. "stag/incident"
    Count  int64   // leaf count at signing time
    Head   string  // hex chain head at signing time
}

type SignedCheckpoint struct {
    Checkpoint
    KeyID string   // hex(sha256(pubkey)[:8]) — which key signed
    Sig   string   // base64(Ed25519 signature over canonical(Checkpoint))
}

func Sign(priv ed25519.PrivateKey, cp Checkpoint) SignedCheckpoint
func VerifySigned(pub ed25519.PublicKey, sc SignedCheckpoint, log io.Reader) (VerifyResult, error)
```

- **Signature scheme: Ed25519** (`crypto/ed25519`, stdlib). Deterministic per RFC 8032 (no per-signature RNG
  to get wrong — ECDSA's nonce-reuse footgun is absent), 32-byte public key, 64-byte signature. This is also
  the scheme the Go transparency/checkpoint ecosystem (`note`, sumdb) uses. P-256 stays at rung 3 where the
  ProofLayer connector needs it — the two layers sign different things and need not share a scheme.
- **Signed bytes: a fixed canonical serialization** of `(origin, count, head)` — a short note-like text
  (`stag-checkpoint/v1\norigin <o>\ncount <n>\nhead <h>\n`). Legible as an audit artifact, no dependency,
  reproducible. (Reusing `stag.CanonicalHash` over the three fields is the alternative; text is chosen for
  human-readability of the sealed statement.)
- **`VerifySigned`** does three things: (1) rung-1 `Verify(log)` to confirm the chain and get its head/count;
  (2) check `sc.Head == chain head` and `sc.Count == chain count` (the checkpoint describes THIS log, not a
  truncated/extended one); (3) `ed25519.Verify(pub, canonical(sc.Checkpoint), sig)`. Any failure → error.

## Key management (the operational surface)

- **Generation:** a `stag-incident keygen -out <dir>/<name>` subcommand writes `<name>.key` (the private
  seed, mode 0600) and `<name>.pub` (the public key). Ed25519 keys are tiny; PEM or raw-base64, stdlib only.
- **The private key is a secret** — it lives at a **gitignored** path (consistent with how `.env.local`
  already holds the API key) and is NEVER committed. The **public key is not secret** — it ships with the
  deployment / is handed to verifiers out-of-band.
- **Config (system wiring):** a `signing` block —
  ```yaml
  signing:
    key: deploy/keys/stag.key     # PRIVATE (gitignored); absent → unsigned, rung 1 only
    pub: deploy/keys/stag.pub     # PUBLIC (for verify)
    origin: stag/incident
  ```
  Absent `signing.key` → the runtime stays rung 1 (no checkpoint) — signing is strictly additive.

## When it signs, and how `verify` uses it

- **Sealing:** when a signing key is configured, the log segment is checkpointed + signed on **Close** (end
  of a `run`/`serve`) and on demand via a `stag-incident checkpoint` subcommand. The signed checkpoint is
  written to a sidecar `events.jsonl.checkpoint` (latest head; overwritten each seal).
- **`verify` extension:** `stag-incident verify` still checks the chain (rung 1); when a checkpoint sidecar
  and a trusted public key are present it additionally verifies the signature and that the chain head matches
  the checkpoint. A present-but-invalid checkpoint, or a head that has moved past the last signed checkpoint
  without re-signing (stale seal), is reported — the append-after-seal case is detectable, not silent.

## The by-hand ladder for rung 2 (what the build will do)

Spec pair (`SignedEgress` + `SignedEgressTest`) → red → green → **fuzz the sign/verify invariant** → harden.
The fuzzable property extends U19's chain-integrity: for a fuzzed event sequence signed with a fresh key,
(1) `VerifySigned(pub, Sign(priv, cp), log)` accepts and returns the right head/count; (2) **any single-byte
tamper to the log, the checkpoint fields, or the signature makes `VerifySigned` reject**; (3) a checkpoint
signed by a **different** key is rejected under the trusted `pub`; (4) Ed25519 signing is deterministic, so a
re-sign is byte-identical. Plus tables: keygen round-trips; a missing/short key fails closed; a stale
checkpoint (head advanced) is detected; an absent key leaves the runtime at rung 1 unchanged.

## Rung 3 preview (deferred): the transparency connector

Rung 3 submits the **signed checkpoint** (the rung-2 artifact) to an external transparency log — ProofLayer
(the sibling engine: "RFC 6962 log and P-256 signing") or Sigstore's Rekor. The external log **witnesses**
the checkpoint (countersigns / remembers it), so a later operator rollback is detectable, and provides
inclusion/consistency proofs (which want a Merkle tree — a refinement over the O(n) hash chain). This is a
**quarantined connector** (its own package, its own P-256 / network dependency isolated like `model/claude`),
reachable asynchronously off the enforcement path (inv 9: egress is best-effort, never gates). Rung 4
(Sigstore Fulcio ambient identity, full X.509 PKI) stays on-demand only.

## Status

**Ratified 2026-07-03 (Curtis): self-sign now (rung 2), connect later (rung 3).** stag holds one
Ed25519 key and signs its own checkpoints — offline-verifiable, no network, no service — and a later,
separate unit adds the ProofLayer/Rekor connector that submits those signed checkpoints for external
witnessing. Pairs with Planning/14 (the ladder) and Planning/13 (ingress); all crypto sits OUTSIDE the
synchronous gate.

**BUILT 2026-07-03 (U21, transcripts/runtime-u21-signed-egress.md).** Rung 2 shipped as designed:
`Checkpoint`/`SignedCheckpoint`, `Sign`/`VerifySigned`, keygen/checkpoint/seal wiring, `verify` extended,
fuzzed 294K execs, PRODUCTION-CLEAN. Live demo isolated its marginal value — a consistent forgery signed by
the wrong key is rejected under the trusted key. **Rung 3 (the ProofLayer/Rekor connector) remains the
deferred follow-on** that closes operator rollback.
