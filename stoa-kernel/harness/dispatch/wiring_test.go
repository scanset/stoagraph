package dispatch

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStagClientCatalogAndRoutes(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/routes", func(w http.ResponseWriter, _ *http.Request) {
		// scale is routed; a same-recipe second route (get_events) must dedupe; an INVALID route
		// (broken recipe) must be excluded; an unrouted recipe (zt_refund_policy) never appears.
		_, _ = w.Write([]byte(`[
			{"tool":"scale_deployment","recipe":"k8s_scale_approval_policy","gateArg":"namespace,replicas,approval_token","valid":true},
			{"tool":"get_pods","recipe":"k8s_read_policy","gateArg":"namespace","valid":true},
			{"tool":"get_events","recipe":"k8s_read_policy","gateArg":"namespace","valid":true},
			{"tool":"x","recipe":"broken","gateArg":"y","valid":false}
		]`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	c := StagClient{BaseURL: ts.URL}

	// catalog = distinct ACTIONABLE recipes (routed + valid); deduped; broken/unrouted excluded
	cat, err := c.Catalog()
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	ids := recipeIDs(cat)
	if !contains(ids, "k8s_scale_approval_policy") || !contains(ids, "k8s_read_policy") {
		t.Fatalf("catalog: got %v, want the routed recipes", ids)
	}
	if contains(ids, "broken") || contains(ids, "zt_refund_policy") {
		t.Fatalf("catalog: got %v, must exclude invalid/unrouted recipes", ids)
	}
	if len(ids) != 2 {
		t.Fatalf("catalog must dedupe same-recipe routes: got %v", ids)
	}

	// a recipe's routes are the tool bindings it governs (the session spec)
	routes, err := c.RoutesForRecipe("k8s_scale_approval_policy")
	if err != nil {
		t.Fatalf("routes: %v", err)
	}
	if len(routes) != 1 || routes[0].Tool != "scale_deployment" || routes[0].GateArg != "namespace,replicas,approval_token" {
		t.Fatalf("routes for recipe: got %+v", routes)
	}

	// an unrouted recipe yields no routes (not actionable — Bind will refuse it)
	if r, _ := c.RoutesForRecipe("zt_refund_policy"); len(r) != 0 {
		t.Errorf("unrouted recipe should have no routes, got %+v", r)
	}

	// a multi-tool session: RoutesForTools returns a route per requested+valid tool (each keeps its
	// own recipe); unknown/invalid tools are skipped.
	tr, err := c.RoutesForTools([]string{"scale_deployment", "get_pods", "get_events", "not_a_tool"})
	if err != nil {
		t.Fatalf("routes for tools: %v", err)
	}
	if len(tr) != 3 {
		t.Fatalf("RoutesForTools: want 3 (scale + 2 reads), got %d: %+v", len(tr), tr)
	}
	byTool := map[string]string{}
	for _, r := range tr {
		byTool[r.Tool] = r.Recipe
	}
	if byTool["scale_deployment"] != "k8s_scale_approval_policy" || byTool["get_pods"] != "k8s_read_policy" {
		t.Errorf("each tool must keep its own recipe: %+v", byTool)
	}
}

// TestProvidersFor asserts the READ-channel resolution (Planning/30): only REQUESTED and ENABLED
// providers are returned; unknown or disabled names are dropped (fail closed, no fabrication).
func TestProvidersFor(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/providers", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[
			{"name":"k8s-kb","kind":"http","config":"{\"url\":\"http://localhost:8095/context\"}","enabled":true},
			{"name":"disabled-kb","kind":"http","config":"{}","enabled":false},
			{"name":"other","kind":"http","config":"{}","enabled":true}
		]`))
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	c := StagClient{BaseURL: ts.URL}

	// empty names -> no READ channel, no HTTP call needed
	if got, err := c.ProvidersFor(nil); err != nil || got != nil {
		t.Fatalf("empty names must yield nil providers: got %v, err %v", got, err)
	}

	// request k8s-kb (enabled) + disabled-kb (disabled) + ghost (unknown)
	got, err := c.ProvidersFor([]string{"k8s-kb", "disabled-kb", "ghost"})
	if err != nil {
		t.Fatalf("ProvidersFor: %v", err)
	}
	if len(got) != 1 || got[0].Name != "k8s-kb" || got[0].Kind != "http" {
		t.Fatalf("must keep only requested+enabled: got %+v", got)
	}
	if got[0].Config == "" {
		t.Errorf("config must be passed through for the daemon to build the provider: %+v", got[0])
	}
}
