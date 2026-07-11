name: SignedEgress
role: component
intent: Rung 2 of the egress trust ladder (Planning/15), added to the egress package: sign the chain head so the audit log is AUTHENTICATED and OFFLINE-VERIFIABLE, not merely tamper-evident relative to a trusted head (rung 1, U19). A Checkpoint is the statement {Origin, Count, Head}; Sign produces a SignedCheckpoint (the checkpoint + a short key fingerprint + an Ed25519 signature over its canonical, domain-separated bytes); VerifySigned confirms the chain (rung 1), that the checkpoint describes THIS log (count+head match), that it was signed by the trusted key (key id), and that the signature verifies. Stdlib-only (crypto/ed25519), NO new dependency. WHAT IT CLOSES: an outsider with no private key cannot forge a log or checkpoint. HONEST CEILING: it does NOT stop the key-holder itself rewriting and re-signing its own past - that needs the external witness of rung 3 (ProofLayer/Rekor connector, deferred). Signing is strictly ADDITIVE: an unsigned log is still a valid rung-1 log.
api:
  - "type Checkpoint struct { Origin string; Count int64; Head string }"
  - "type SignedCheckpoint struct { Checkpoint; KeyID string; Sig string }"
  - func GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey, error)
  - func KeyID(pub ed25519.PublicKey) string
  - func Sign(priv ed25519.PrivateKey, cp Checkpoint) SignedCheckpoint
  - func VerifySigned(pub ed25519.PublicKey, sc SignedCheckpoint, log io.Reader) (VerifyResult, error)
  - func MarshalPrivate(priv ed25519.PrivateKey) []byte
  - func ParsePrivate(b []byte) (ed25519.PrivateKey, error)
  - func MarshalPublic(pub ed25519.PublicKey) []byte
  - func ParsePublic(b []byte) (ed25519.PublicKey, error)
concept: signed checkpoint over the chain head; authenticated + offline-verifiable audit log; one Ed25519 keypair; no external service; the honest ceiling is operator rollback (rung 3).
behavior:
  - "SIGN: Sign(priv, cp) returns a SignedCheckpoint with the same Checkpoint, KeyID == KeyID(the public half of priv), and Sig == base64(Ed25519 signature over signBody(cp)), where signBody is a fixed domain-separated, deterministic serialization of cp (a version-prefixed canonical encoding of Origin/Count/Head). Ed25519 signatures are deterministic (RFC 8032), so Sign on the same cp and key is byte-identical."
  - "KEYID: KeyID(pub) is a short stable hex fingerprint of the public key (e.g. hex(sha256(pub)[:8])); the same pub always yields the same id, different pubs (overwhelmingly) differ."
  - "VERIFYSIGNED ACCEPTS AN HONEST SIGNED LOG: VerifySigned(pub, sc, log) (1) runs Verify(log) (rung 1) to confirm the chain and get its head+count; (2) requires sc.Count == chain count AND sc.Head == chain head (the checkpoint describes exactly this log); (3) requires sc.KeyID == KeyID(pub) (signed by the trusted key); (4) base64-decodes sc.Sig and requires ed25519.Verify(pub, signBody(sc.Checkpoint), sig). On success it returns the VerifyResult (head+count) and a nil error."
  - "VERIFYSIGNED FAILS CLOSED: any of these returns a non-nil error and never a false accept - a broken chain (rung 1 error); a count or head that does not match the log (truncated/extended/rewritten); a KeyID that is not the trusted key; a malformed base64 signature; a signature that does not verify; a signature by a DIFFERENT key than pub; a pub whose length is not ed25519.PublicKeySize (guarded, never panics). VerifySigned never panics on any input."
  - "KEY MARSHAL ROUND-TRIP: MarshalPrivate/ParsePrivate and MarshalPublic/ParsePublic round-trip a key through a stable text encoding (base64 of the raw key material). ParsePrivate and ParsePublic FAIL CLOSED on wrong length or invalid encoding (a non-nil error, never a truncated key). A key produced by GenerateKey marshals and parses back to an equal key that still Signs/Verifies."
  - "ADDITIVE: none of this changes rung 1 - Verify, JSONLSink, Record are untouched. A log with no checkpoint is still a valid rung-1 log; signing is an added seal, not a new log format."
constraints: package egress at workspaces/stag/egress (extends the existing package; import path github.com/scanset/StAG/egress). Depends on stdlib only (crypto/ed25519, crypto/rand, crypto/sha256, encoding/base64, encoding/hex, encoding/json, bytes, errors, fmt, io) plus the existing rung-1 Verify in the same package. No third-party dependency. Rung 3 (submit the signed checkpoint to an external transparency log) is a separate, quarantined connector, deferred.
