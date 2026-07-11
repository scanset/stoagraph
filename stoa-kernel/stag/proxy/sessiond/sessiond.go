// Package sessiond is the stag-proxy v2 daemon surface: a standing HTTP server where each MCP
// session is bound to a dispatcher-chosen recipe (Planning/24 v2, /25). The TRUSTED dispatcher
// POSTs /sessions to bind a session to a set of routes and gets back an opaque token; the UNTRUSTED
// agent connects to /mcp/<token>, and every tool call in that session is gated by THAT session's
// recipe — not a global table. The agent cannot choose its own recipe (the token is minted here and
// the binding is server-side). One daemon owns one downstream + one audit sink, so there is no
// per-run log fork.
package sessiond

// file-kw: session daemon registry token router session-to-recipe streamable-http bind fail-closed no-fork

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/scanset/stoagraph/stoa-kernel/stag/auth"
	"github.com/scanset/stoagraph/stoa-kernel/stag/provider"
	"github.com/scanset/stoagraph/stoa-kernel/stag/proxy"
	"github.com/scanset/stoagraph/stoa-kernel/stag/proxy/mcpgate"
	"github.com/scanset/stoagraph/stoa-kernel/stag/router"
)

// sessionIdleTimeout closes an MCP session (its streamable-HTTP transport) after this long with no
// client request, so a standing daemon does not accumulate abandoned protocol sessions. The token→
// recipe binding in the Registry is separate and ephemeral (cleared on daemon restart); per-token
// TTL is a v1.1 hardening.
const sessionIdleTimeout = 30 * time.Minute

// boundSession is one session's binding: the recipe router (ACT channel) and the context providers
// (READ channel). Both are chosen by the trusted dispatcher at bind time; the untrusted agent gets
// only the token.
type boundSession struct {
	router    proxy.Router
	providers []provider.ContextProvider
}

// Registry holds active session bindings — each a boundSession (the recipe router the dispatcher
// chose, plus its READ-channel providers). In-memory and EPHEMERAL (v1): a session dies with the
// daemon; a dropped session just re-dispatches. This is the Session entity of Planning/25.
type Registry struct {
	mu       sync.RWMutex
	sessions map[string]boundSession
}

// NewRegistry returns an empty session registry.
func NewRegistry() *Registry { return &Registry{sessions: map[string]boundSession{}} }

// Create builds a session router from route specs (fail-closed: it needs at least one route that
// resolves) and binds it — with its READ-channel providers — under a fresh opaque token. The recipe
// and provider choice belong to the caller (the trusted dispatcher); the untrusted agent only ever
// receives the token.
func (r *Registry) Create(specs []router.Spec, providers []provider.ContextProvider, loadRecipe func(string) ([]byte, error)) (string, []router.RouteError, error) {
	resolved := router.Build(specs, loadRecipe)
	if len(resolved.Router) == 0 {
		return "", resolved.Errors, errors.New("no valid routes in binding")
	}
	tok, err := mintToken()
	if err != nil {
		return "", nil, err
	}
	r.mu.Lock()
	r.sessions[tok] = boundSession{router: resolved.Router, providers: providers}
	r.mu.Unlock()
	return tok, resolved.Errors, nil
}

// lookup resolves a token to its session binding.
func (r *Registry) lookup(tok string) (boundSession, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	bs, ok := r.sessions[tok]
	return bs, ok
}

// Count reports the number of live sessions (for /health and tests).
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}

func mintToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Deps are the per-daemon gate ingredients shared across every session — everything EXCEPT the
// per-session Routes: the egress sink (the one owned audit log), the approval store + escalation
// hook (Stage 5), and the shared downstream to forward cleared calls to.
type Deps struct {
	Sink       proxy.Sink
	Approvals  proxy.Approvals
	OnEscalate func(ctx context.Context, n proxy.PendingNotice)
	Downstream *mcp.ClientSession
	Tools      []*mcp.Tool
	LoadRecipe func(string) ([]byte, error)
	// RecordRead audits one READ crossing (Planning/30). May be nil (recording best-effort). The
	// READ channel is label+record, so this is the "record" half; it is separate from Sink (the
	// hash-chained ACT-release log) because a read is not a release.
	RecordRead func(ctx context.Context, ev provider.ReadEvent)
	// Auth guards POST /sessions with the `dispatch` role (Planning/31): binding a session CHOOSES
	// the recipe that will govern it, so an unauthenticated binder could simply pick the most
	// permissive recipe — the "the agent cannot choose its own recipe" invariant would collapse.
	// A NIL Auth fails CLOSED. Note /mcp/<token> is deliberately NOT guarded by this: the opaque
	// session token IS the agent's credential, and handing the untrusted agent a control-plane
	// bearer would be exactly backwards.
	Auth *auth.Authenticator
}

// Handler is the daemon's HTTP surface:
//   - POST /sessions  (TRUSTED — the dispatcher) binds a session to routes, returns {token, path}.
//   - /mcp/<token>    (UNTRUSTED — the agent) is the gated MCP endpoint; the token selects the
//     session's recipe. An unknown/absent token returns 400 (fail closed — no session, no gate).
func Handler(reg *Registry, deps Deps) http.Handler {
	mux := http.NewServeMux()

	// POST /sessions is the TRUSTED binder — it chooses the recipe. `dispatch` role required.
	mux.HandleFunc("POST /sessions", deps.Auth.Guard(auth.RoleDispatch)(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Routes []struct {
				Tool    string `json:"tool"`
				Recipe  string `json:"recipe"`
				GateArg string `json:"gateArg"`
			} `json:"routes"`
			// Context is the READ-channel binding (Planning/30): the provider specs (already resolved
			// upstream from the config DB) this session may read. Optional — absent => no READ channel.
			Context []struct {
				Name   string `json:"name"`
				Kind   string `json:"kind"`
				Config string `json:"config"`
			} `json:"context"`
		}
		body, _ := io.ReadAll(r.Body)
		if json.Unmarshal(body, &req) != nil || len(req.Routes) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "need routes:[{tool,recipe,gateArg}]"})
			return
		}
		specs := make([]router.Spec, 0, len(req.Routes))
		for _, rt := range req.Routes {
			specs = append(specs, router.Spec{Tool: rt.Tool, Recipe: rt.Recipe, GateArg: rt.GateArg})
		}
		// Build the READ-channel providers. A provider that won't build (unsupported kind, bad config)
		// is DROPPED from the session and logged — fail closed, never fabricate a source.
		providers := make([]provider.ContextProvider, 0, len(req.Context))
		for _, c := range req.Context {
			p, perr := provider.FromConfig(c.Name, c.Kind, c.Config)
			if perr != nil {
				log.Printf("session bind: dropping context provider %q: %v", c.Name, perr)
				continue
			}
			providers = append(providers, p)
		}
		tok, rerrs, err := reg.Create(specs, providers, deps.LoadRecipe)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "routeErrors": routeErrView(rerrs)})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"token": tok, "path": "/mcp/" + tok})
	}))

	// One StreamableHTTPHandler serves all sessions; getServer is called once per NEW MCP session
	// and returns the gating server bound to THAT token's recipe. NO control-plane bearer here: the
	// opaque session token in the path IS the untrusted agent's credential (Planning/31).
	streamable := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		tok := strings.TrimPrefix(req.URL.Path, "/mcp/")
		bs, ok := reg.lookup(tok)
		if !ok {
			return nil // -> 400: no binding, no gate, nothing served
		}
		gate := proxy.Gate{Routes: bs.router, Sink: deps.Sink, Approvals: deps.Approvals, OnEscalate: deps.OnEscalate}
		read := mcpgate.ReadChannel{Providers: bs.providers, Record: deps.RecordRead}
		return mcpgate.NewGatingServer(gate, deps.Downstream, toolsFor(bs.router, deps.Tools), read)
	}, &mcp.StreamableHTTPOptions{SessionTimeout: sessionIdleTimeout})
	mux.Handle("/mcp/", streamable)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "sessions": reg.Count()})
	})
	return mux
}

// toolsFor filters the downstream tools to those the session's router governs, so the agent's
// tools/list reflects ITS recipe — it never sees a tool its session can't use.
func toolsFor(rt proxy.Router, all []*mcp.Tool) []*mcp.Tool {
	out := make([]*mcp.Tool, 0, len(all))
	for _, t := range all {
		if _, ok := rt[t.Name]; ok {
			out = append(out, t)
		}
	}
	return out
}

func routeErrView(errs []router.RouteError) []map[string]string {
	out := make([]map[string]string, 0, len(errs))
	for _, e := range errs {
		out = append(out, map[string]string{"tool": e.Tool, "recipe": e.Recipe, "error": e.Err})
	}
	return out
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
