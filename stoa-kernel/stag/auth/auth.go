// Package auth is the control-plane authentication for stag (Planning/31): bearer tokens with ROLES.
//
// The load-bearing rule this package exists to enforce: the `dispatch` role (the ORCHESTRATOR — a
// machine) may bind sessions and POLL approval status, but can NEVER approve. An orchestrator that
// could approve its own escalations would make the human-in-the-loop gate decorative, and the whole
// product thesis — the gate is separate from the orchestrator precisely so the orchestrator cannot
// authorize itself — would be a lie while every test still passed. Hence roles, not one shared token.
//
// Fail closed everywhere: an unknown role, an unset token, a missing/!bearer header -> 401.
package auth

// file-kw: control-plane auth bearer roles constant-time fail-closed tokens dispatch cannot approve

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
)

// The four control-plane roles. `dispatch` is the only one a machine ever holds.
const (
	RoleAdmin    = "admin"    // policy CRUD (recipes, routes, mcp-servers, providers) — the human
	RoleApprove  = "approve"  // mints the signed release (approve/deny) — the HUMAN ONLY, never the harness
	RoleDispatch = "dispatch" // POST /sessions + catalog reads + approval POLL — the orchestrator
	RoleOperator = "operator" // harness-serve's own API (models, event map, dispatch/run) — the human
)

// envFor maps a role to the env var that overrides its token (container / k8s secrets take precedence
// over the file, so a deployment never has to bake secrets into an image).
var envFor = map[string]string{
	RoleAdmin:    "STAG_ADMIN_TOKEN",
	RoleApprove:  "STAG_APPROVE_TOKEN",
	RoleDispatch: "STAG_DISPATCH_TOKEN",
	RoleOperator: "HARNESS_OPERATOR_TOKEN",
}

// Tokens is one independent secret per role. Separate secrets are the point: see the package doc.
type Tokens struct {
	Admin    string `json:"admin"`
	Approve  string `json:"approve"`
	Dispatch string `json:"dispatch"`
	Operator string `json:"operator"`
}

// Get returns the secret for a role, or "" for an unknown/unset role — which is FAIL CLOSED: Verify
// rejects an empty expectation, so an unconfigured role admits NOBODY (it never admits everybody).
func (t Tokens) Get(role string) string {
	switch role {
	case RoleAdmin:
		return t.Admin
	case RoleApprove:
		return t.Approve
	case RoleDispatch:
		return t.Dispatch
	case RoleOperator:
		return t.Operator
	}
	return ""
}

// Only returns a copy of t keeping ONLY the named roles; every other secret is zeroed.
//
// This closes a hole that the HTTP-layer role check does NOT: the gate rejects a `dispatch` bearer on
// an approve route, but a compromised orchestrator would never bother SENDING `dispatch` — it would
// send the `approve` token it happened to be holding, because the tokens file contains all four.
// Co-locating the secrets defeats a role split just as surely as sharing one token.
//
// So every consumer narrows immediately at load and keeps nothing it is not entitled to:
//
//	stag-serve     all four   (it is the ISSUER, and verifies admin/approve/dispatch)
//	stag-proxy     dispatch   (it only guards POST /sessions)
//	harness-serve  dispatch + operator  — NEVER approve. It waits on a human; it can never be one.
//
// In a container, the orchestrator gets its two secrets via env and never mounts the tokens file at
// all, so `approve` is not even on its filesystem. This is the in-process half of the same rule.
func (t Tokens) Only(roles ...string) Tokens {
	var out Tokens
	for _, r := range roles {
		switch r {
		case RoleAdmin:
			out.Admin = t.Admin
		case RoleApprove:
			out.Approve = t.Approve
		case RoleDispatch:
			out.Dispatch = t.Dispatch
		case RoleOperator:
			out.Operator = t.Operator
		}
	}
	return out
}

// Verify reports whether presented is the token for role, compared in CONSTANT TIME (no timing
// oracle on a control-plane secret). An unset expectation always fails.
func (t Tokens) Verify(role, presented string) bool {
	want := t.Get(role)
	if want == "" || presented == "" {
		return false // fail closed
	}
	return subtle.ConstantTimeCompare([]byte(want), []byte(presented)) == 1
}

// withEnv overlays env-var overrides onto a token set.
func (t Tokens) withEnv() Tokens {
	if v := os.Getenv(envFor[RoleAdmin]); v != "" {
		t.Admin = v
	}
	if v := os.Getenv(envFor[RoleApprove]); v != "" {
		t.Approve = v
	}
	if v := os.Getenv(envFor[RoleDispatch]); v != "" {
		t.Dispatch = v
	}
	if v := os.Getenv(envFor[RoleOperator]); v != "" {
		t.Operator = v
	}
	return t
}

// Load reads the tokens file and applies env overrides. It does NOT generate — services that only
// CONSUME tokens (the daemon, harness-serve) must fail rather than invent a secret nobody else knows.
func Load(path string) (Tokens, error) {
	var t Tokens
	b, err := os.ReadFile(path)
	if err != nil {
		// env alone is a legitimate source (containers): if it fully populates, the file is optional.
		if t = (Tokens{}).withEnv(); t.Admin != "" || t.Approve != "" || t.Dispatch != "" || t.Operator != "" {
			return t, nil
		}
		return Tokens{}, fmt.Errorf("control-plane tokens: %w (start stag-serve first to generate them, or set the STAG_*_TOKEN env vars)", err)
	}
	if err := json.Unmarshal(b, &t); err != nil {
		return Tokens{}, fmt.Errorf("control-plane tokens %s: %w", path, err)
	}
	return t.withEnv(), nil
}

// LoadOrGenerate loads the token set, generating and persisting a fresh one (0600) on first run —
// the same pattern as the ed25519 approval key, so a fresh `docker compose up` is CLOSED BY DEFAULT
// with zero setup. Returns generated=true when it wrote a new file. stag-serve owns generation; the
// daemon and harness-serve only Load.
func LoadOrGenerate(path string) (t Tokens, generated bool, err error) {
	// No path => ENV ONLY, and never touch disk. This is the container case: each service is injected
	// exactly the secrets it is entitled to, and NO tokens file exists anywhere — so `approve` is not
	// merely unused by the orchestrator, it is not on its filesystem to be read. Minting a file here
	// would put every secret back in one place and undo the whole point.
	if path == "" {
		t = Tokens{}.withEnv()
		if t.Admin == "" && t.Approve == "" && t.Dispatch == "" && t.Operator == "" {
			return Tokens{}, false, fmt.Errorf("control-plane tokens: no -tokens path and no STAG_*_TOKEN env vars")
		}
		return t, false, nil
	}
	if b, rerr := os.ReadFile(path); rerr == nil {
		if err := json.Unmarshal(b, &t); err != nil {
			return Tokens{}, false, fmt.Errorf("control-plane tokens %s: %w", path, err)
		}
		return t.withEnv(), false, nil
	}
	for _, p := range []*string{&t.Admin, &t.Approve, &t.Dispatch, &t.Operator} {
		s, err := secret()
		if err != nil {
			return Tokens{}, false, err
		}
		*p = s
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return Tokens{}, false, err
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
		return Tokens{}, false, fmt.Errorf("control-plane tokens %s: %w", path, err)
	}
	return t.withEnv(), true, nil
}

// secret mints one 32-byte random token (hex). Not brute-forceable; no rate limiting needed in v1.
func secret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Authenticator guards the control plane. Disabled is the -dev-no-auth escape hatch: it bypasses
// EVERY check, so it must never be the default and never be set in a deployment.
type Authenticator struct {
	Tokens   Tokens
	Disabled bool
}

// Allow reports whether r presents a bearer token for ONE of roles.
func (a *Authenticator) Allow(r *http.Request, roles ...string) bool {
	if a == nil {
		return false // no authenticator wired -> fail closed, never open
	}
	if a.Disabled {
		return true
	}
	presented := bearer(r)
	if presented == "" {
		return false
	}
	for _, role := range roles {
		if a.Tokens.Verify(role, presented) {
			return true
		}
	}
	return false
}

// Guard wraps a handler so only a caller bearing one of roles reaches it. Everything else gets a 401
// and an audit line — a security product should say when it was probed.
func (a *Authenticator) Guard(roles ...string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if a.Allow(r, roles...) {
				next(w, r)
				return
			}
			log.Printf("control-plane 401: %s %s from %s (requires %s)",
				r.Method, r.URL.Path, r.RemoteAddr, strings.Join(roles, "|"))
			w.Header().Set("WWW-Authenticate", `Bearer realm="stag control plane"`)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized: the control plane requires a bearer token for this role"}` + "\n"))
		}
	}
}

// bearer extracts the token from an `Authorization: Bearer <token>` header ("" when absent/malformed).
func bearer(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}
