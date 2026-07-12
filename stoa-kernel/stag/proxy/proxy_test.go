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

type spySink struct{ recs []stag.DecisionRecord }

func (s *spySink) Record(_ context.Context, d stag.DecisionRecord) error {
	s.recs = append(s.recs, d)
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

// TestRecordsEveryDecision — the audit chain records EVERY decision, not only the permitted ones. A
// blocked attempt is the evidence the control worked, and "did anything try?" is the question the log
// exists to answer. A denied decision is recorded, and it carries NO release: nothing crossed.
func TestRecordsEveryDecision(t *testing.T) {
	router, _, _ := policyRouter(t)
	sink := &spySink{}
	g := proxy.Gate{Routes: router, Sink: sink}
	ctx := context.Background()

	g.Decide(ctx, proxy.ToolCall{Tool: "write_note", Args: map[string]string{"text": "hello"}})
	if len(sink.recs) != 1 {
		t.Fatalf("an allowed call must be recorded, got %d leaves", len(sink.recs))
	}
	if d := sink.recs[0]; d.Verdict != "allow" || !d.Forwarded || len(d.Events) == 0 {
		t.Errorf("an allowed call must record a forwarded decision carrying its release: %+v", d)
	}

	g.Decide(ctx, proxy.ToolCall{Tool: "write_note", Args: map[string]string{"text": "nope"}})
	if len(sink.recs) != 2 {
		t.Fatalf("a DENIED call must still be recorded — the blocked attempt is the evidence")
	}
	if d := sink.recs[1]; d.Verdict != "deny" || d.Forwarded || len(d.Events) != 0 {
		t.Errorf("a denied call releases nothing, so it must record no crossing: %+v", d)
	}

	g.Decide(ctx, proxy.ToolCall{Tool: "delete_everything", Args: map[string]string{}})
	if len(sink.recs) != 3 {
		t.Fatalf("an UNROUTED tool call must be recorded — reaching for an ungranted tool is the most suspicious call of all")
	}
	if d := sink.recs[2]; d.Verdict != "deny" || d.Forwarded || len(d.Events) != 0 || d.Fault == "" {
		t.Errorf("unrouted must record deny + a reason and no release: %+v", d)
	}
}

const twoArgPolicySrc = `recipe: two_arg_policy
version: 1
rules:
  owner.allowed:
    kind: set_membership
    set: ["scanset"]
  repo.allowed:
    kind: set_membership
    set: ["stoagraph"]
steps:
  - {id: po, kind: propose, out: owner}
  - {id: pr, kind: propose, out: repo}
  - {id: so, kind: sink, in: owner, field: gh.owner, sensitivity: authoritative, rule: owner.allowed, actor: "policy:gh"}
  - {id: sr, kind: sink, in: repo,  field: gh.repo,  sensitivity: authoritative, rule: repo.allowed, actor: "policy:gh"}
`

// TestDeniedMultiArgRecordsNoRelease is the regression test for a real bug found against the live GitHub
// MCP server. A multi-arg recipe evaluates EVERY sink, so a denied call can still have a sibling sink
// that individually cleared: owner=mallory fails, but repo=stoagraph passes its rule. The gate used to
// record that passing sink as a release — putting a crossing in the tamper-evident log that NEVER
// HAPPENED. An auditor would have concluded the agent read a repo the gate actually blocked.
func TestDeniedMultiArgRecordsNoRelease(t *testing.T) {
	p, err := recipe.Parse([]byte(twoArgPolicySrc))
	if err != nil {
		t.Fatalf("two-arg policy must parse: %v", err)
	}
	sink := &spySink{}
	g := proxy.Gate{
		Routes: proxy.Router{"get_file_contents": {Recipe: p.Recipe, RecipeHash: p.SemanticHash, GateArg: "owner,repo"}},
		Sink:   sink,
	}

	// owner is NOT allowed; repo IS. The whole call must deny, and nothing may be recorded as released.
	d := g.Decide(context.Background(), proxy.ToolCall{
		Tool: "get_file_contents",
		Args: map[string]string{"owner": "mallory", "repo": "stoagraph"},
	})
	if d.Verdict != stag.Deny || d.Forward {
		t.Fatalf("a failing owner must deny the whole call: %+v", d)
	}
	if len(sink.recs) != 1 {
		t.Fatalf("the denied attempt must be recorded exactly once, got %d", len(sink.recs))
	}
	rec := sink.recs[0]
	if rec.Verdict != "deny" || rec.Forwarded {
		t.Fatalf("recorded decision must be a non-forwarded deny: %+v", rec)
	}
	if len(rec.Events) != 0 {
		t.Fatalf("BUG: a denied call recorded %d release(s) — the log would assert a crossing that never happened: %+v",
			len(rec.Events), rec.Events)
	}

	// and the permitted call DOES release both crossings
	g.Decide(context.Background(), proxy.ToolCall{
		Tool: "get_file_contents",
		Args: map[string]string{"owner": "scanset", "repo": "stoagraph"},
	})
	if len(sink.recs) != 2 {
		t.Fatalf("the allowed call must be recorded")
	}
	if ok := sink.recs[1]; !ok.Forwarded || len(ok.Events) != 2 {
		t.Fatalf("an allowed multi-arg call releases BOTH crossings: %+v", ok)
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
