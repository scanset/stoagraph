package record

// file-kw: canonical hash sha256 deterministic json sorted keys stable sensitive attestation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// kw: canonical json sorted keys
func CanonicalJSON(v any) ([]byte, error) {
	return json.Marshal(v) // encoding/json sorts map keys at every level
}

// kw: canonical hash sha256 hex
func CanonicalHash(v any) (string, error) {
	b, err := CanonicalJSON(v)
	if err != nil {
		return "", err // fail closed: no hash of partial bytes
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
