package egress

// file-kw: signed checkpoint ed25519 head seal authenticated offline-verifiable rung2 no-pki keyid

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// sigContext domain-separates a checkpoint signature from any other signed bytes.
const sigContext = "stag-checkpoint/v1\n"

// kw: checkpoint origin count head statement about the chain head
type Checkpoint struct {
	Origin string `json:"origin"`
	Count  int64  `json:"count"`
	Head   string `json:"head"`
}

// kw: signed checkpoint keyid base64 ed25519 sig
type SignedCheckpoint struct {
	Checkpoint
	KeyID string `json:"key_id"`
	Sig   string `json:"sig"`
}

// signBody is the exact, deterministic, domain-separated byte string that is
// signed and verified: a version prefix + the canonical JSON of the checkpoint
// (a struct, so json.Marshal is stable and never errors for these field types).
func signBody(cp Checkpoint) []byte {
	b, _ := json.Marshal(cp)
	return append([]byte(sigContext), b...)
}

// kw: generate ed25519 keypair
func GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}

// kw: key id short fingerprint of public key
func KeyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:8])
}

// kw: sign checkpoint ed25519 deterministic domain-separated
func Sign(priv ed25519.PrivateKey, cp Checkpoint) SignedCheckpoint {
	pub := priv.Public().(ed25519.PublicKey)
	sig := ed25519.Sign(priv, signBody(cp)) // deterministic (RFC 8032)
	return SignedCheckpoint{
		Checkpoint: cp,
		KeyID:      KeyID(pub),
		Sig:        base64.StdEncoding.EncodeToString(sig),
	}
}

// kw: verify signed checkpoint chain + head match + keyid + signature fail-closed
func VerifySigned(pub ed25519.PublicKey, sc SignedCheckpoint, log io.Reader) (VerifyResult, error) {
	if len(pub) != ed25519.PublicKeySize {
		return VerifyResult{}, fmt.Errorf("egress: public key is %d bytes, want %d", len(pub), ed25519.PublicKeySize)
	}
	// 1. the chain itself (rung 1)
	res, err := Verify(log)
	if err != nil {
		return VerifyResult{}, err
	}
	// 2. the checkpoint describes THIS log, not a truncated/extended/rewritten one
	if sc.Count != res.Count {
		return VerifyResult{}, fmt.Errorf("egress: checkpoint count %d != chain count %d", sc.Count, res.Count)
	}
	if sc.Head != res.Head {
		return VerifyResult{}, errors.New("egress: checkpoint head does not match the chain head")
	}
	// 3. signed by the trusted key
	if sc.KeyID != KeyID(pub) {
		return VerifyResult{}, fmt.Errorf("egress: checkpoint key %s is not the trusted key %s", sc.KeyID, KeyID(pub))
	}
	// 4. the signature
	sig, err := base64.StdEncoding.DecodeString(sc.Sig)
	if err != nil {
		return VerifyResult{}, fmt.Errorf("egress: signature encoding: %w", err)
	}
	if !ed25519.Verify(pub, signBody(sc.Checkpoint), sig) {
		return VerifyResult{}, errors.New("egress: signature does not verify")
	}
	return res, nil
}

// approvalContext domain-separates an approval release signature from any other signed bytes
// (a checkpoint sig and an approval sig over the same string are not interchangeable).
const approvalContext = "stag-approval/v1\n"

// SignApproval mints a signed release token: an ed25519 signature over an action fingerprint,
// base64-encoded. The token authorizes EXACTLY that action (the fingerprint binds tool+args), is
// deterministic (RFC 8032), and is verifiable offline by anyone holding the public key.
func SignApproval(priv ed25519.PrivateKey, fingerprint string) string {
	sig := ed25519.Sign(priv, append([]byte(approvalContext), fingerprint...))
	return base64.StdEncoding.EncodeToString(sig)
}

// VerifyApproval checks a release token against an action fingerprint (offline audit / defence in
// depth). Fail-closed on any decode or length error.
func VerifyApproval(pub ed25519.PublicKey, fingerprint, token string) bool {
	if len(pub) != ed25519.PublicKeySize {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return false
	}
	return ed25519.Verify(pub, append([]byte(approvalContext), fingerprint...), sig)
}

// kw: marshal private key base64 seed
func MarshalPrivate(priv ed25519.PrivateKey) []byte {
	return encodeKey(priv.Seed()) // the 32-byte seed reconstructs the full key
}

// kw: parse private key fail-closed length
func ParsePrivate(b []byte) (ed25519.PrivateKey, error) {
	seed, err := decodeKey(b, ed25519.SeedSize)
	if err != nil {
		return nil, fmt.Errorf("egress: private key: %w", err)
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// kw: marshal public key base64
func MarshalPublic(pub ed25519.PublicKey) []byte {
	return encodeKey(pub)
}

// kw: parse public key fail-closed length
func ParsePublic(b []byte) (ed25519.PublicKey, error) {
	raw, err := decodeKey(b, ed25519.PublicKeySize)
	if err != nil {
		return nil, fmt.Errorf("egress: public key: %w", err)
	}
	return ed25519.PublicKey(raw), nil
}

// kw: encode key base64 line
func encodeKey(raw []byte) []byte {
	return []byte(base64.StdEncoding.EncodeToString(raw) + "\n")
}

// kw: decode key base64 length-check fail-closed
func decodeKey(b []byte, want int) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(string(bytes.TrimSpace(b)))
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}
	if len(raw) != want {
		return nil, fmt.Errorf("wrong length: %d bytes, want %d", len(raw), want)
	}
	return raw, nil
}
