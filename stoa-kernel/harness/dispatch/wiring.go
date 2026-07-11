package dispatch

// file-kw: wiring stag-serve catalog routes-for-recipe session binder daemon post-sessions token

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RouteSpec is one tool→recipe→gateArg binding — what a session is built from.
type RouteSpec struct {
	Tool    string `json:"tool"`
	Recipe  string `json:"recipe"`
	GateArg string `json:"gateArg"`
}

// ProviderSpec is one resolved context provider — the READ-channel half of a session binding
// (Planning/30). The daemon builds a live provider from {name, kind, config}; the agent never sees it.
type ProviderSpec struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Config string `json:"config"`
}

// StagClient reads the routable policy from stag-serve (the console's API): the recipe catalog for
// the dispatch model, and the routes that a chosen recipe governs (to build a session).
//
// Token is the control-plane `dispatch` secret (Planning/31) — the ORCHESTRATOR's role. It admits
// catalog reads and the approval POLL. It deliberately CANNOT approve or write policy: an
// orchestrator able to approve its own escalations would make the human gate decorative.
type StagClient struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func (c StagClient) httpc() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 10 * time.Second}
}

// Catalog lists the ACTIONABLE recipes for the dispatch model — the distinct recipes that have a
// valid route (i.e. can actually govern a session). A recipe with no route can't be bound, so
// offering it would only let the model pick an unbindable target; the catalog excludes them. Recipes
// carry no description today, so WhenToUse is the name (the deterministic event map, which names
// recipes explicitly, is the primary path). Suitable as a Dispatcher.Catalog.
func (c StagClient) Catalog() ([]Recipe, error) {
	var routes []struct {
		Recipe string `json:"recipe"`
		Valid  bool   `json:"valid"`
	}
	if err := c.get("/api/routes", &routes); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := make([]Recipe, 0)
	for _, r := range routes {
		if r.Valid && !seen[r.Recipe] {
			seen[r.Recipe] = true
			out = append(out, Recipe{ID: r.Recipe, WhenToUse: r.Recipe})
		}
	}
	return out, nil
}

// RoutesForRecipe returns the route bindings a recipe governs (the session's routes). A recipe with
// no route is not actionable — the session would have nothing to gate (fail closed at bind).
func (c StagClient) RoutesForRecipe(recipe string) ([]RouteSpec, error) {
	var routes []struct {
		Tool    string `json:"tool"`
		Recipe  string `json:"recipe"`
		GateArg string `json:"gateArg"`
		Valid   bool   `json:"valid"`
	}
	if err := c.get("/api/routes", &routes); err != nil {
		return nil, err
	}
	out := make([]RouteSpec, 0, 1)
	for _, r := range routes {
		if r.Recipe == recipe && r.Valid {
			out = append(out, RouteSpec{Tool: r.Tool, Recipe: r.Recipe, GateArg: r.GateArg})
		}
	}
	return out, nil
}

// RoutesForTools returns the route bindings for a SET of tools (a multi-tool session profile). Each
// tool keeps its own recipe; a tool with no valid route is silently skipped (fail closed at the gate).
func (c StagClient) RoutesForTools(tools []string) ([]RouteSpec, error) {
	want := make(map[string]bool, len(tools))
	for _, t := range tools {
		want[t] = true
	}
	var routes []struct {
		Tool    string `json:"tool"`
		Recipe  string `json:"recipe"`
		GateArg string `json:"gateArg"`
		Valid   bool   `json:"valid"`
	}
	if err := c.get("/api/routes", &routes); err != nil {
		return nil, err
	}
	out := make([]RouteSpec, 0, len(tools))
	for _, r := range routes {
		if want[r.Tool] && r.Valid {
			out = append(out, RouteSpec{Tool: r.Tool, Recipe: r.Recipe, GateArg: r.GateArg})
		}
	}
	return out, nil
}

// ProvidersFor resolves a set of context-provider NAMES to their specs (the READ-channel binding,
// Planning/30), keeping only ENABLED providers. An unknown or disabled name is silently dropped —
// fail closed: the session simply gets no READ channel for it, never a fabricated source. Empty names
// -> no providers (no READ channel).
func (c StagClient) ProvidersFor(names []string) ([]ProviderSpec, error) {
	if len(names) == 0 {
		return nil, nil
	}
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}
	var provs []struct {
		Name    string `json:"name"`
		Kind    string `json:"kind"`
		Config  string `json:"config"`
		Enabled bool   `json:"enabled"`
	}
	if err := c.get("/api/providers", &provs); err != nil {
		return nil, err
	}
	out := make([]ProviderSpec, 0, len(names))
	for _, p := range provs {
		if want[p.Name] && p.Enabled {
			out = append(out, ProviderSpec{Name: p.Name, Kind: p.Kind, Config: p.Config})
		}
	}
	return out, nil
}

func (c StagClient) get(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token) // the `dispatch` role
	}
	resp, err := c.httpc().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("stag %s: 401 — the orchestrator's `dispatch` control-plane token is missing or wrong", path)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("stag %s: HTTP %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Binder binds a session on the stag-proxy DAEMON: POST /sessions {routes} → an opaque token whose
// /mcp/<token> endpoint gates against exactly those routes.
type Binder struct {
	DaemonURL string
	Token     string // the control-plane `dispatch` secret — POST /sessions requires it (Planning/31)
	HTTP      *http.Client
}

// Bind registers a session for the given routes (ACT) and context providers (READ) and returns the
// /mcp/<token> endpoint the agent connects to. Fails if no routes (nothing to gate) or the daemon
// rejects the binding (bad recipe). Providers may be empty — a session with no READ channel.
func (b Binder) Bind(ctx context.Context, routes []RouteSpec, providers []ProviderSpec) (endpoint, token string, err error) {
	if len(routes) == 0 {
		return "", "", fmt.Errorf("no routes to bind (recipe is not routed to any tool)")
	}
	body, _ := json.Marshal(map[string]any{"routes": routes, "context": providers})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.DaemonURL+"/sessions", bytes.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if b.Token != "" {
		req.Header.Set("Authorization", "Bearer "+b.Token) // the `dispatch` role
	}
	hc := b.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("bind session: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		return "", "", fmt.Errorf("bind session: 401 — the orchestrator's `dispatch` control-plane token is missing or wrong")
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("bind session: HTTP %d: %s", resp.StatusCode, raw)
	}
	var out struct {
		Token string `json:"token"`
		Path  string `json:"path"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || out.Token == "" {
		return "", "", fmt.Errorf("bind session: bad response: %s", raw)
	}
	return b.DaemonURL + out.Path, out.Token, nil
}
