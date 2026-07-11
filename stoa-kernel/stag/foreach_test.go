package stag

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func allowedRule() *ReleaseRule {
	return &ReleaseRule{Kind: RuleSetMembership, Set: []string{"restart", "scale", "clear"}}
}

// propose(list) -> foreach(as=item) -> authoritative sink gated by the allowed set.
func foreachRecipe() Recipe {
	return Recipe{Steps: []Step{
		{Id: "p", Kind: NodePropose, Out: "plan"},
		{Id: "fe", Kind: NodeForeach, In: "plan", As: "item"},
		{Id: "apply", Kind: NodeSink, In: "item", Field: "exec.action", Sensitivity: SinkAuthoritative, Rule: allowedRule(), RuleID: "action.allowed", Actor: "policy:x"},
	}}
}

func TestForeachAllAllowed(t *testing.T) {
	r := foreachRecipe()
	res := Eval(r, `["restart","scale"]`, "h")
	if res.Verdict != Allow || res.Fault != "" {
		t.Fatalf("all-allowed batch: %+v", res)
	}
	if len(res.Events) != 2 {
		t.Fatalf("want 2 release events, got %d", len(res.Events))
	}
	seen := map[int64]bool{}
	for _, e := range res.Events {
		if e.SubjectClass != Untrusted || e.RecipeHash != "h" {
			t.Errorf("event: %+v", e)
		}
		if seen[e.Ordering] {
			t.Errorf("release events must have distinct Ordering per element: %d", e.Ordering)
		}
		seen[e.Ordering] = true
	}
}

func TestForeachOneDeniedDeniesBatch(t *testing.T) {
	res := Eval(foreachRecipe(), `["restart","bad_cmd"]`, "h")
	if res.Verdict != Deny {
		t.Errorf("one denied element must deny the batch: %v", res.Verdict)
	}
	if len(res.Events) != 1 {
		t.Errorf("only the allowed element crosses: %d events", len(res.Events))
	}
}

func TestForeachEmptyList(t *testing.T) {
	res := Eval(foreachRecipe(), `[]`, "h")
	if res.Verdict != Allow || len(res.Events) != 0 || len(res.Sinks) != 0 {
		t.Errorf("empty list must be Allow with no crossings: %+v", res)
	}
}

func TestForeachFailsClosed(t *testing.T) {
	r := foreachRecipe()
	cases := []string{`not-json`, `[1,2]`, `{"a":1}`, `"just a string"`}
	for _, p := range cases {
		res := Eval(r, p, "h")
		if res.Fault == "" || res.Verdict != Deny || len(res.Events) != 0 {
			t.Errorf("proposal %q must fault: %+v", p, res)
		}
	}
	// over cap
	big, _ := json.Marshal(make([]string, foreachCap+1))
	if res := Eval(r, string(big), "h"); res.Fault == "" || res.Verdict != Deny {
		t.Errorf("over-cap list must fault: %+v", res)
	}
	// nested foreach in the body -> fault
	nested := Recipe{Steps: []Step{
		{Id: "p", Kind: NodePropose, Out: "plan"},
		{Id: "fe", Kind: NodeForeach, In: "plan", As: "item"},
		{Id: "fe2", Kind: NodeForeach, In: "item", As: "sub"},
		{Id: "apply", Kind: NodeSink, In: "sub", Field: "x", Sensitivity: SinkAuthoritative, Rule: allowedRule(), RuleID: "r", Actor: "a"},
	}}
	if res := Eval(nested, `["[\"restart\"]"]`, "h"); res.Fault == "" {
		t.Errorf("nested foreach must fault: %+v", res)
	}
}

func TestNodeKindForeachParse(t *testing.T) {
	k, err := ParseNodeKind("foreach")
	if err != nil || k != NodeForeach || k.String() != "foreach" {
		t.Errorf("foreach node kind: k=%v err=%v str=%q", k, err, k.String())
	}
	if _, err := ParseNodeKind("bogus"); err == nil {
		t.Error("unknown kind must still error")
	}
}

// regression: a non-foreach recipe evaluates exactly as before the refactor.
func TestNonForeachUnchanged(t *testing.T) {
	r := Recipe{Steps: []Step{
		{Id: "p", Kind: NodePropose, Out: "v"},
		{Id: "s", Kind: NodeSink, In: "v", Field: "exec", Sensitivity: SinkAuthoritative, Rule: allowedRule(), RuleID: "r", Actor: "a"},
	}}
	res := Eval(r, "restart", "h")
	if res.Verdict != Allow || len(res.Events) != 1 || res.Events[0].Ordering != 1 {
		t.Errorf("non-foreach recipe changed: %+v", res)
	}
}

func FuzzForeach(f *testing.F) {
	f.Add([]byte{0, 1, 2})
	f.Add([]byte{})
	f.Add([]byte{3, 3, 3}) // all denied
	f.Add(make([]byte, foreachCap+5))
	vocab := []string{"restart", "scale", "clear", "NOPE"} // index 3 is denied
	r := foreachRecipe()
	f.Fuzz(func(t *testing.T, data []byte) {
		elems := make([]string, 0, len(data))
		for _, b := range data {
			elems = append(elems, vocab[int(b)%4])
		}
		arr, err := json.Marshal(elems)
		if err != nil {
			t.Skip()
		}
		res := Eval(r, string(arr), "h")

		if len(elems) > foreachCap {
			if res.Fault == "" || res.Verdict != Deny {
				t.Fatalf("over-cap must fault: %d elems", len(elems))
			}
			return
		}
		// recompute independently
		allAllowed, wantEvents := true, 0
		for _, e := range elems {
			if strings.HasPrefix(e, "NOPE") { // the one denied token
				allAllowed = false
			} else {
				wantEvents++
			}
		}
		want := Deny
		if allAllowed {
			want = Allow
		}
		if res.Verdict != want {
			t.Fatalf("verdict %v, want %v (elems %v)", res.Verdict, want, elems)
		}
		if len(res.Events) != wantEvents {
			t.Fatalf("events %d, want %d (elems %v)", len(res.Events), wantEvents, elems)
		}
		if res2 := Eval(r, string(arr), "h"); !reflect.DeepEqual(res, res2) {
			t.Fatalf("nondeterministic")
		}
	})
}
