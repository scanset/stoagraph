package stag

import "testing"

// EvalArgs binds each `propose out: X` from args[X] — so one recipe can decide from several
// named arguments. This recipe gates arg `a` (a gate that must equal "pass") and arg `b` (a
// sink that must equal "ok"); swapping which input is bad changes the verdict, proving the
// two are bound DISTINCTLY from their named args.
func TestEvalArgsBindsNamedInputs(t *testing.T) {
	ra := ReleaseRule{Kind: RuleSetMembership, Set: []string{"pass"}}
	rb := ReleaseRule{Kind: RuleSetMembership, Set: []string{"ok"}}
	r := Recipe{Steps: []Step{
		{Id: "pa", Kind: NodePropose, Out: "a"},
		{Id: "pb", Kind: NodePropose, Out: "b"},
		{Id: "ga", Kind: NodeGate, In: "a", Rule: &ra, RuleID: "ra"},
		{Id: "sb", Kind: NodeSink, In: "b", Field: "f.b", Sensitivity: SinkAuthoritative, Rule: &rb, RuleID: "rb", Actor: "x"},
	}}
	cases := []struct {
		a, b string
		want Verdict
	}{
		{"pass", "ok", Allow}, // both inputs satisfy their own rule
		{"nope", "ok", Deny},  // a fails its gate -> Deny  (a bound from args["a"])
		{"pass", "bad", Deny}, // b fails its sink -> Deny  (b bound from args["b"])
	}
	for _, c := range cases {
		if got := EvalArgs(r, map[string]string{"a": c.a, "b": c.b}, "h").Verdict; got != c.want {
			t.Errorf("EvalArgs(a=%q,b=%q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
	// Backward compat: single-arg Eval binds the ONE proposal to every propose. Here that
	// makes a=b="pass", so sink b (needs "ok") denies.
	if v := Eval(r, "pass", "h").Verdict; v != Deny {
		t.Errorf("single-arg Eval a=b=\"pass\": want Deny, got %v", v)
	}
}
