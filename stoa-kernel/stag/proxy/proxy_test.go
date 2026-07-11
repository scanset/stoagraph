package proxy_test

import (
	"context"
	"reflect"
	"testing"

	stag "github.com/scanset/stoagraph/stoa-kernel/stag"
	"github.com/scanset/stoagraph/stoa-kernel/stag/proxy"
	"github.com/scanset/stoagraph/stoa-kernel/stag/recipe"
)

const policySrc = `recipe: write_note_policy
version: 1
rules:
  note.allowed:
    kind: set_membership
    set: ["hello", "status-ok", "deploy-done"]
steps:
  - id: propose_text
    kind: propose
    out: text
  - id: apply
    kind: sink
    in: text
    field: mcp.write_note.text
    sensitivity: authoritative
    rule: note.allowed
    actor: "policy:mcp_proxy"
`

func policyRouter(t testing.TB) (proxy.Router, stag.Recipe, string) {
	t.Helper()
	p, err := recipe.Parse([]byte(policySrc))
	if err != nil {
		t.Fatalf("policy recipe must parse: %v", err)
	}
	r := proxy.Router{
		"write_note": {Recipe: p.Recipe, RecipeHash: p.SemanticHash, GateArg: "text"},
	}
	return r, p.Recipe, p.SemanticHash
}

type spySink struct{ events []stag.ReleaseEvent }

func (s *spySink) Record(_ context.Context, ev stag.ReleaseEvent) error {
	s.events = append(s.events, ev)
	return nil
}

func TestForwardsRoutedAllowed(t *testing.T) {
	router, _, _ := policyRouter(t)
	g := proxy.Gate{Routes: router}
	d := g.Decide(context.Background(), proxy.ToolCall{Tool: "write_note", Args: map[string]string{"text": "hello"}})
	if d.Verdict != stag.Allow || !d.Forward || d.Value != "hello" {
		t.Fatalf("routed+allowed must forward: %+v", d)
	}
	if len(d.Events) == 0 {
		t.Errorf("an authoritative crossing should emit a release event: %+v", d)
	}
}

func TestUnknownToolFailsClosed(t *testing.T) {
	router, _, _ := policyRouter(t)
	g := proxy.Gate{Routes: router}
	d := g.Decide(context.Background(), proxy.ToolCall{Tool: "delete_everything", Args: map[string]string{}})
	if d.Verdict != stag.Deny || d.Forward || d.Fault == "" {
		t.Errorf("unknown tool must fail closed (deny, no forward, a reason): %+v", d)
	}
}

func TestDeniedDoesNotForward(t *testing.T) {
	router, _, _ := policyRouter(t)
	g := proxy.Gate{Routes: router}
	d := g.Decide(context.Background(), proxy.ToolCall{Tool: "write_note", Args: map[string]string{"text": "rm -rf /"}})
	if d.Verdict != stag.Deny || d.Forward {
		t.Errorf("a value outside the allowed set must be denied and not forwarded: %+v", d)
	}
	if len(d.Events) != 0 {
		t.Errorf("a denied crossing emits no release event: %+v", d)
	}
}

func TestRecordsEvents(t *testing.T) {
	router, _, _ := policyRouter(t)
	sink := &spySink{}
	g := proxy.Gate{Routes: router, Sink: sink}

	g.Decide(context.Background(), proxy.ToolCall{Tool: "write_note", Args: map[string]string{"text": "hello"}})
	if len(sink.events) == 0 {
		t.Errorf("allowed call must record the crossing")
	}
	before := len(sink.events)
	g.Decide(context.Background(), proxy.ToolCall{Tool: "write_note", Args: map[string]string{"text": "nope"}})
	if len(sink.events) != before {
		t.Errorf("denied call must record nothing new")
	}
}

func FuzzForwardIffCleared(f *testing.F) {
	f.Add("write_note", "hello")
	f.Add("write_note", "rm -rf /")
	f.Add("unknown_tool", "hello")
	f.Add("", "")
	f.Fuzz(func(t *testing.T, tool, arg string) {
		// build the fixed router fresh (recipe.Parse is deterministic)
		p, err := recipe.Parse([]byte(policySrc))
		if err != nil {
			t.Fatalf("recipe: %v", err)
		}
		router := proxy.Router{"write_note": {Recipe: p.Recipe, RecipeHash: p.SemanticHash, GateArg: "text"}}
		g := proxy.Gate{Routes: router}

		call := proxy.ToolCall{Tool: tool, Args: map[string]string{"text": arg}}
		d := g.Decide(context.Background(), call)

		// (1) forward => routed AND kernel independently Allows
		if d.Forward {
			route, ok := router[tool]
			if !ok {
				t.Fatalf("FORWARD OF UNROUTED TOOL %q", tool)
			}
			if stag.Eval(route.Recipe, arg, route.RecipeHash).Verdict != stag.Allow {
				t.Fatalf("FORWARD OF NON-ALLOWED call tool=%q arg=%q verdict=%v", tool, arg, d.Verdict)
			}
		}
		// (2) unrouted tool: deny, no forward
		if _, ok := router[tool]; !ok {
			if d.Forward || d.Verdict != stag.Deny {
				t.Fatalf("unrouted tool must deny+not-forward: %+v", d)
			}
		}
		// (4) determinism
		if d2 := g.Decide(context.Background(), call); !reflect.DeepEqual(d, d2) {
			t.Fatalf("nondeterministic decision")
		}
	})
}
