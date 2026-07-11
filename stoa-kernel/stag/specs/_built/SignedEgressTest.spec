name: SignedEgressTest
role: test
intent: Verify rung-2 signed checkpoints: Sign/VerifySigned round-trip on an honest signed log, fail closed on every tamper (log bytes, checkpoint fields, signature, wrong key, malformed pub), key marshal/parse round-trips and fails closed on junk, and signing is deterministic. A fuzz drives arbitrary event sequences, signs the head with a fixed-seed key, and asserts the honest signed log verifies while any single-byte tamper to the log or the signature - and any checkpoint-field mutation or wrong key - is rejected.
api:
  - func TestSignVerifyRoundTrip(t *testing.T)
  - func TestVerifySignedFailsClosed(t *testing.T)
  - func TestKeyMarshalRoundTrip(t *testing.T)
  - func FuzzSignedCheckpoint(f *testing.F)
prelude: "A fixed 32-byte seed builds a deterministic Ed25519 key via ed25519.NewKeyFromSeed so tests and fuzz are reproducible. A helper records N events to a bytes.Buffer via NewJSONLSink and returns the bytes plus the sink's head/count, from which a Checkpoint is built and signed."
behavior:
  - "SIGN/VERIFY ROUND-TRIP: record 3 events; build Checkpoint{Origin:\"stoagraph/test\", Count, Head} from the sink; sc := Sign(priv, cp). VerifySigned(pub, sc, log) returns a nil error with Head==sink head and Count==3. sc.KeyID == KeyID(pub). Signing the same cp again is byte-identical (deterministic)."
  - "VERIFYSIGNED FAILS CLOSED (table), each returns a non-nil error: (a) flip a byte in the log; (b) sc.Count+1; (c) change sc.Head; (d) flip a character in sc.Sig; (e) sc.Sig set to non-base64 junk; (f) verify under a DIFFERENT key's public half; (g) sc.KeyID changed to some other value; (h) a public key of the wrong length (must not panic)."
  - "KEY MARSHAL ROUND-TRIP: priv/pub from GenerateKey; ParsePrivate(MarshalPrivate(priv)) equals priv and ParsePublic(MarshalPublic(pub)) equals pub, and the round-tripped keys still Sign/VerifySigned. ParsePrivate and ParsePublic each return a non-nil error on truncated bytes and on invalid base64; the returned key is unusable, not partial."
  - "FUZZ FuzzSignedCheckpoint(data []byte, pos uint16): build 0..5 events from data, record to a log, read head+count, sign Checkpoint{origin, count, head} with the fixed-seed key. ASSERT: (1) VerifySigned(pub, sc, log) is nil-error with matching head+count; (2) signing again is byte-identical (determinism); (3) if the log is non-empty, flipping the byte at pos%len makes VerifySigned reject; (4) flipping a byte of the decoded signature makes VerifySigned reject; (5) VerifySigned under a second, different fixed-seed key rejects; (6) never panics. Seed with empty, one event, several events, and adversarial byte fragments."
constraints: package egress_test (external test); depends on the egress package and stdlib (bytes, crypto/ed25519, encoding/base64, strings, testing). No third-party deps, no network, no files (keys are in-memory from fixed seeds).
