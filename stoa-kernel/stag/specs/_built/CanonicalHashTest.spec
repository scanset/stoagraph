name: CanonicalHashTest
role: test
intent: Verify the two load-bearing properties of the hash primitive - stability under map reordering and sensitivity to any single-field change - by table and by fuzz, plus the hash format and the fail-closed error path. If the hash were unstable, every attestation would be unverifiable; if it were insensitive, a tampered record would hash identically to the original.
api:
  - func TestCanonicalHash(t *testing.T)
  - func FuzzCanonicalHash(f *testing.F)
behavior:
  - "STABILITY: CanonicalHash(map[string]any{\"a\":1,\"b\":2}) == CanonicalHash(map[string]any{\"b\":2,\"a\":1}) (same content, different literal order); hashing the same map twice returns the same value."
  - "NESTED STABILITY: CanonicalHash(map[string]any{\"o\": map[string]any{\"a\":1,\"b\":2}}) == CanonicalHash(map[string]any{\"o\": map[string]any{\"b\":2,\"a\":1}}) (keys sorted at every level)."
  - "SENSITIVITY: a changed value (map{\"a\":1} vs map{\"a\":2}), an added key (map{\"a\":1} vs map{\"a\":1,\"b\":2}), and a removed key each produce a DIFFERENT hash."
  - "FORMAT + DEFINITION: the returned hash matches ^[0-9a-f]{64}$; and it equals hex.EncodeToString(sha256.Sum256(cj)) where cj is CanonicalJSON of the same value (recomputed independently in the test)."
  - "ERROR: CanonicalHash(make(chan int)) returns a non-nil error and an empty string."
  - "FUZZ FuzzCanonicalHash: fuzz two string keys k1,k2 and an int64 n; return early if k1==k2 (the property needs distinct keys). Build m1 := map[string]any{k1:n, k2:\"x\"} and m2 := map[string]any{k2:\"x\", k1:n} (same content, different insertion order); assert CanonicalHash(m1)==CanonicalHash(m2) and that hashing m1 twice agrees (STABILITY). Build m3 := map[string]any{k1:n, k2:\"y\"} (one value changed); assert CanonicalHash(m3) != CanonicalHash(m1) (SENSITIVITY). Assert every returned hash has length 64. Ignore errors only if both sides error identically (they will not for these string/int maps)."
constraints: package main; standard library only (testing, crypto/sha256, encoding/hex).
