package serve

// file-kw: oauth start callback status pending pkce sign-in handlers downstream authorization-code

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"time"

	"github.com/scanset/stoagraph/stoa-kernel/stag/oauth"
)

// pendingOAuth is an in-flight sign-in held between /api/oauth/start and the browser callback.
type pendingOAuth struct {
	server   string
	verifier string
	cfg      oauth.Config
	created  time.Time
}

// serverOAuthConfig is the optional per-server hint (mcp_servers.oauth_config JSON) for providers that
// do NOT support dynamic client registration and need a pre-registered client / explicit scopes.
type serverOAuthConfig struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	Scopes       []string `json:"scopes"`
}

func (s *Server) redirectURI() string { return s.PublicURL + oauth.CallbackPath }

// handleOAuthStart discovers the server's authorization server, (dynamically) registers a client, mints
// PKCE + state, and returns the authorization URL for the operator's browser to open. Admin-guarded.
func (s *Server) handleOAuthStart(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("server")
	if name == "" {
		writeJSON(w, http.StatusBadRequest, errObj("server query param required"))
		return
	}
	if s.Store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errObj("no config store"))
		return
	}
	sv, err := s.Store.GetMCPServer(r.Context(), name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errObj("unknown server: "+name))
		return
	}
	if sv.Transport != "http" {
		writeJSON(w, http.StatusBadRequest, errObj("oauth applies to http servers only"))
		return
	}
	cfg, err := oauth.Discover(r.Context(), nil, sv.Target)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errObj("oauth discovery failed: "+err.Error()))
		return
	}
	if sv.OAuthConfig != "" { // operator-provided client / scopes for non-DCR providers
		var oc serverOAuthConfig
		if json.Unmarshal([]byte(sv.OAuthConfig), &oc) == nil {
			if oc.ClientID != "" {
				cfg.ClientID = oc.ClientID
			}
			if oc.ClientSecret != "" {
				cfg.ClientSecret = oc.ClientSecret
			}
			if len(oc.Scopes) > 0 {
				cfg.Scopes = oc.Scopes
			}
		}
	}
	cfg, err = oauth.Register(r.Context(), nil, cfg, s.redirectURI(), "StoaGraph ("+name+")")
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errObj(err.Error()))
		return
	}
	if cfg.ClientID == "" {
		writeJSON(w, http.StatusBadRequest, errObj("this provider has no dynamic registration — set client_id in the server's oauth config"))
		return
	}
	verifier, challenge := oauth.PKCE()
	state := oauth.NewState()
	s.stashPending(state, pendingOAuth{server: name, verifier: verifier, cfg: cfg, created: time.Now()})
	writeJSON(w, http.StatusOK, map[string]any{"authUrl": cfg.AuthCodeURL(s.redirectURI(), state, challenge)})
}

// handleOAuthCallback is the provider's redirect target. It is PUBLIC (the provider redirects the
// operator's browser here and cannot carry our bearer); the unguessable state — minted only by an
// admin-authenticated start — is what authenticates the completion. It exchanges the code for tokens,
// persists them gate-side, and returns a self-closing page.
func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		oauthResultPage(w, "Sign-in was declined", strings2(e, q.Get("error_description")))
		return
	}
	p, ok := s.takePending(q.Get("state"))
	if !ok {
		oauthResultPage(w, "Sign-in expired", "Unknown or expired request — start the sign-in again from the console.")
		return
	}
	tok, err := p.cfg.Exchange(r.Context(), nil, s.redirectURI(), q.Get("code"), p.verifier)
	if err != nil {
		oauthResultPage(w, "Sign-in failed", err.Error())
		return
	}
	if err := s.OAuth.Save(p.server, oauth.State{Config: p.cfg, Tokens: tok}); err != nil {
		oauthResultPage(w, "Sign-in failed", "could not persist tokens: "+err.Error())
		return
	}
	oauthResultPage(w, "Signed in to "+p.server, "You can close this window.")
}

// handleOAuthStatus reports whether a server has a usable token, and when it expires. Read-guarded.
func (s *Server) handleOAuthStatus(w http.ResponseWriter, r *http.Request) {
	st, err := s.OAuth.Load(r.URL.Query().Get("server"))
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"authorized": false})
		return
	}
	out := map[string]any{"authorized": st.Authorized(), "hasRefresh": st.Tokens.RefreshToken != ""}
	if !st.Tokens.Expiry.IsZero() {
		out["expiresAt"] = st.Tokens.Expiry
	}
	writeJSON(w, http.StatusOK, out)
}

// --- in-flight sign-in bookkeeping (in-memory, self-pruning) ----------------------------------------

const oauthPendingTTL = 15 * time.Minute

func (s *Server) stashPending(state string, p pendingOAuth) {
	s.oauthMu.Lock()
	defer s.oauthMu.Unlock()
	if s.oauthPending == nil {
		s.oauthPending = map[string]pendingOAuth{}
	}
	for k, v := range s.oauthPending { // prune abandoned flows
		if time.Since(v.created) > oauthPendingTTL {
			delete(s.oauthPending, k)
		}
	}
	s.oauthPending[state] = p
}

func (s *Server) takePending(state string) (pendingOAuth, bool) {
	s.oauthMu.Lock()
	defer s.oauthMu.Unlock()
	p, ok := s.oauthPending[state]
	if ok {
		delete(s.oauthPending, state)
	}
	return p, ok
}

func strings2(a, b string) string {
	if b == "" {
		return a
	}
	return a + ": " + b
}

// oauthResultPage renders a minimal self-closing page for the popup. Both fields are escaped — the
// message can carry provider-supplied error text.
func oauthResultPage(w http.ResponseWriter, title, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!doctype html><meta charset=utf-8><title>%s</title>`+
		`<body style="font:15px system-ui;background:#0b0d12;color:#e6e8ec;display:grid;place-items:center;height:100vh;margin:0">`+
		`<div style="text-align:center;max-width:28rem;padding:2rem">`+
		`<div style="font-size:1.15rem;font-weight:600;margin-bottom:.5rem">%s</div>`+
		`<div style="color:#9aa3b2">%s</div></div>`+
		`<script>setTimeout(function(){window.close()},1200)</script>`,
		html.EscapeString(title), html.EscapeString(title), html.EscapeString(msg))
}
