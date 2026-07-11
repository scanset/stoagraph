package dispatch

import (
	"context"
	"encoding/json"
	"testing"
)

func TestGate(t *testing.T) {
	valid := []string{"k8s_incident_policy", "zt_refund_policy"}
	cases := []struct {
		id, conf string
		want     GateDecision
	}{
		{"k8s_incident_policy", "high", GateMatch},
		{"zt_refund_policy", "medium", GateMatch},
		{"k8s_incident_policy", "low", GateFallback}, // low confidence refused
		{"not_a_recipe", "high", GateFallback},       // off-list refused (model can't invent)
		{"none", "high", GateFallback},               // explicit none
		{"", "high", GateFallback},                   // empty
	}
	for _, c := range cases {
		if got := Gate(c.id, c.conf, valid); got != c.want {
			t.Errorf("Gate(%q,%q) = %v, want %v", c.id, c.conf, got, c.want)
		}
	}
}

func event(t *testing.T, js string) Event {
	t.Helper()
	var e Event
	if err := json.Unmarshal([]byte(js), &e); err != nil {
		t.Fatalf("bad event json: %v", err)
	}
	return e
}

func TestEventMapMatch(t *testing.T) {
	m := EventMap{
		{ID: "pd-incident", Match: map[string]string{"source": "pagerduty", "event.type": "incident.triggered"}, Recipe: "k8s_incident_policy"},
		{ID: "stripe-refund", Match: map[string]string{"source": "stripe", "event.type": "charge.dispute.created"}, Recipe: "zt_refund_policy"},
		{ID: "bad", Match: map[string]string{}, Recipe: "never"}, // empty predicate must never match
	}
	// full match on nested dotted path
	if d, ok := m.Match(event(t, `{"source":"pagerduty","event":{"type":"incident.triggered"}}`)); !ok || d.ID != "pd-incident" {
		t.Errorf("pagerduty incident: got (%v,%v), want pd-incident", d.ID, ok)
	}
	// second definition
	if d, ok := m.Match(event(t, `{"source":"stripe","event":{"type":"charge.dispute.created"}}`)); !ok || d.ID != "stripe-refund" {
		t.Errorf("stripe refund: got (%v,%v)", d.ID, ok)
	}
	// partial match (one field wrong) -> no match
	if _, ok := m.Match(event(t, `{"source":"pagerduty","event":{"type":"incident.resolved"}}`)); ok {
		t.Error("a partial predicate match must NOT match")
	}
	// missing field -> no match; empty-predicate definition never rescues it
	if _, ok := m.Match(event(t, `{"source":"zendesk"}`)); ok {
		t.Error("unmatched event must not match (empty predicate is fail-closed)")
	}
}

type stubRouter struct{ res RouteResult }

func (s stubRouter) Route(context.Context, Event, []Recipe) (RouteResult, error) { return s.res, nil }
func (s stubRouter) Name() string                                                { return "stub" }

func TestDispatchDeterministicFirst(t *testing.T) {
	// a deterministic definition matches -> its recipe, NO model call (router would pick differently).
	d := Dispatcher{
		Map:    EventMap{{ID: "pd", Match: map[string]string{"source": "pagerduty"}, Recipe: "k8s_incident_policy"}},
		Router: stubRouter{RouteResult{RecipeID: "WRONG", Confidence: "high"}},
		Catalog: func() ([]Recipe, error) {
			return []Recipe{{ID: "k8s_incident_policy"}, {ID: "WRONG"}}, nil
		},
	}
	dec, err := d.Dispatch(context.Background(), event(t, `{"source":"pagerduty"}`))
	if err != nil {
		t.Fatal(err)
	}
	if dec.Mode != "deterministic" || dec.RecipeID != "k8s_incident_policy" || dec.Definition != "pd" {
		t.Fatalf("deterministic-first: got %+v", dec)
	}
}

func TestDispatchModelFallbackAndGate(t *testing.T) {
	cat := func() ([]Recipe, error) { return []Recipe{{ID: "zt_refund_policy"}}, nil }
	base := Dispatcher{Map: nil, Catalog: cat} // no deterministic match -> model route

	// model names a valid recipe at good confidence -> dispatch
	base.Router = stubRouter{RouteResult{RecipeID: "zt_refund_policy", Confidence: "high"}}
	if dec, _ := base.Dispatch(context.Background(), event(t, `{"source":"stripe"}`)); dec.Mode != "model" || dec.RecipeID != "zt_refund_policy" {
		t.Fatalf("model route: got %+v", dec)
	}

	// low confidence -> Gate rejects -> none
	base.Router = stubRouter{RouteResult{RecipeID: "zt_refund_policy", Confidence: "low"}}
	if dec, _ := base.Dispatch(context.Background(), event(t, `{}`)); dec.Dispatched() {
		t.Fatalf("low confidence must not dispatch: %+v", dec)
	}

	// off-list recipe (model tried to invent) -> Gate rejects -> none
	base.Router = stubRouter{RouteResult{RecipeID: "made_up", Confidence: "high"}}
	if dec, _ := base.Dispatch(context.Background(), event(t, `{}`)); dec.Dispatched() {
		t.Fatalf("off-list recipe must not dispatch: %+v", dec)
	}

	// no router at all -> none (deterministic-only deployment)
	base.Router = nil
	if dec, _ := base.Dispatch(context.Background(), event(t, `{}`)); dec.Mode != "none" {
		t.Fatalf("no router: want none, got %+v", dec)
	}
}

func TestParseRouteLenient(t *testing.T) {
	// tolerate a code fence + prose around the JSON
	rr := parseRoute("Sure!\n```json\n{\"recipe_id\": \"zt_refund_policy\", \"confidence\": \"High\"}\n```\n")
	if rr.RecipeID != "zt_refund_policy" || rr.Confidence != "high" {
		t.Errorf("lenient parse: got %+v", rr)
	}
	// garbage -> fail-closed defaults
	if rr := parseRoute("no json here"); rr.RecipeID != "none" || rr.Confidence != "low" {
		t.Errorf("garbage parse: got %+v", rr)
	}
}
