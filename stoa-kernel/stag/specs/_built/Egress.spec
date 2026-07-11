name: Egress
role: component
intent: The v1 egress layer (rung 1 of the trust ladder, Planning/14) - a hash-chained, tamper-evident JSONL event log behind the broker.EventSink seam, with NO keys and NO PKI. JSONLSink.Record appends each ReleaseEvent as one newline-delimited Leaf carrying its sequence, the prior leaf's hash (prev_hash), and its own chain hash; Verify reads a log back and confirms the whole chain is internally consistent, returning the head hash and leaf count. THE LOAD-BEARING PROPERTY (chain integrity): Verify accepts an honest log, and ANY single-byte mutation - an edited field, a reordered/dropped/inserted leaf, a rewritten hash, corrupted bytes - makes Verify reject. This is tamper-evidence RELATIVE TO A TRUSTED HEAD (the record is tamper-evident): it does NOT stop a total rewrite by whoever controls the store - that gap is closed by the deferred signing+anchor rungs (Planning/14), delegated to the ProofLayer/Rekor connector. JSONLSink satisfies broker.EventSink structurally (Record(ctx, stag.ReleaseEvent) error) so it drops into the broker with no change.
api:
  - "type Leaf struct { Seq int64; PrevHash string; Event stag.ReleaseEvent; Hash string }"
  - "type VerifyResult struct { Head string; Count int64 }"
  - "const GenesisHash = \"\""
  - "type JSONLSink struct { ... unexported fields ... }"
  - func NewJSONLSink(w io.Writer) *JSONLSink
  - func ResumeJSONLSink(w io.Writer, head string, seq int64) *JSONLSink
  - func (s *JSONLSink) Record(ctx context.Context, ev stag.ReleaseEvent) error
  - func (s *JSONLSink) Head() string
  - func (s *JSONLSink) Count() int64
  - func Verify(r io.Reader) (VerifyResult, error)
concept: hash-chained append-only event log; tamper-evidence relative to a trusted head; no PKI; the EventSink egress rung 1.
behavior:
  - "RECORD APPENDS A CHAINED LEAF: Record(ctx, ev) writes exactly one line - a JSON Leaf - with Seq = the current 0-based sequence, PrevHash = the prior leaf's Hash (GenesisHash \"\" for the first), Event = ev verbatim, and Hash = CanonicalHash(seq, prev_hash, event_hash) where event_hash = ev.Hash(). It then advances the head to Hash and increments the sequence. The write is the last mutating step: if the underlying Writer errors (or ev.Hash errors), Record returns the error and does NOT advance head or sequence - fail closed, so the in-memory chain matches exactly what was actually written."
  - "HEAD AND COUNT: Head() returns the hash of the last leaf written (GenesisHash before any). Count() returns the number of leaves written. Both are concurrency-safe (a mutex serializes Record/Head/Count)."
  - "VERIFY ACCEPTS AN HONEST LOG: Verify(r) reads newline-delimited leaves and checks, for each in order: seq is the expected 0-based increment; prev_hash equals the previous leaf's hash (GenesisHash for leaf 0); and hash equals the recomputed CanonicalHash(seq, prev_hash, ev.Hash()). On success it returns VerifyResult{Head: the last leaf's hash, Count: the number of leaves}; an empty log (no non-blank lines) yields {Head: GenesisHash, Count: 0} and a nil error. For ANY log produced by a JSONLSink, Verify's Head equals the sink's Head() and Count equals its Count()."
  - "VERIFY REJECTS TAMPERING: any mutation of an honest log makes Verify return a non-nil error - an edited event field (recomputed event hash no longer matches the leaf hash), a rewritten leaf Hash, a changed prev_hash (chain broken), a reordered / dropped / inserted leaf (seq or prev mismatch), or bytes that no longer decode as a Leaf. Verify NEVER panics on any input and never returns a nil error for a tampered log."
  - "DETERMINISTIC: recording the same sequence of events to two independent sinks yields byte-identical output; CanonicalHash is deterministic (sorted-key JSON, U5)."
  - "RESUME: ResumeJSONLSink(w, head, seq) continues an existing chain - the next Record uses prev_hash = head and the given starting seq - so a restarted process that Verifies its existing log (obtaining head + count) can append without breaking the chain, and Verify over the concatenation still accepts. NewJSONLSink(w) is exactly ResumeJSONLSink(w, GenesisHash, 0)."
constraints: package egress at workspaces/stag/egress (public; import path github.com/scanset/StAG/egress). Depends on the stag root (ReleaseEvent, ReleaseEvent.Hash, CanonicalHash) and stdlib (bufio, bytes, context, encoding/json, fmt, io, sync). No keys, no signing, no network - those are the deferred rungs (Planning/14). Satisfies broker.EventSink structurally without importing broker.
