package stag

import "testing"

// NodeExit is a pure terminal: it halts the walk, adds no verdict and records no crossing.
// A path reaching exit yields whatever verdicts accumulated before it — and exit HALTS, so
// steps physically after it on that path do not run (the property composition relies on).
func TestExitTerminates(t *testing.T) {
	rule := ReleaseRule{Kind: RuleSetMembership, Set: []string{"ok"}}
	// propose -> authoritative sink (clears "ok") -> exit ; then a SECOND authoritative sink
	// that would DENY. exit must halt before it: verdict Allow, exactly one crossing.
	r := Recipe{Steps: []Step{
		{Id: "p", Kind: NodePropose, Out: "v"},
		{Id: "s1", Kind: NodeSink, In: "v", Field: "f.one", Sensitivity: SinkAuthoritative, Rule: &rule, RuleID: "r", Actor: "a"},
		{Id: "done", Kind: NodeExit},
		{Id: "s2", Kind: NodeSink, In: "v", Field: "f.two", Sensitivity: SinkAuthoritative},
	}}
	res := Eval(r, "ok", "h")
	if res.Fault != "" {
		t.Fatalf("unexpected fault: %q", res.Fault)
	}
	if res.Verdict != Allow {
		t.Errorf("verdict = %v, want Allow (exit halts before the denying sink)", res.Verdict)
	}
	if len(res.Events) != 1 || res.Events[0].TargetField != "f.one" {
		t.Errorf("events = %+v, want exactly the f.one crossing", res.Events)
	}
	if len(res.Sinks) != 1 {
		t.Errorf("sink outcomes = %d, want 1 (s2 after exit never runs)", len(res.Sinks))
	}
}

// An exit as the very first reachable step yields a vacuous Allow: nothing gated, nothing
// crossed (AndAll of no verdicts).
func TestExitOnlyIsVacuousAllow(t *testing.T) {
	r := Recipe{Steps: []Step{{Id: "done", Kind: NodeExit}}}
	res := Eval(r, "anything", "h")
	if res.Verdict != Allow || len(res.Events) != 0 || res.Fault != "" {
		t.Errorf("exit-only: %+v, want Allow / 0 events / no fault", res)
	}
}
