package router_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/scanset/stoagraph/stoa-kernel/stag/recipe"
	"github.com/scanset/stoagraph/stoa-kernel/stag/router"
)

func policy(name, tool, arg string) string {
	return fmt.Sprintf(`recipe: %s
version: 1
rules:
  r.allowed:
    kind: set_membership
    set: ["ok"]
steps:
  - id: p
    kind: propose
    out: v
  - id: s
    kind: sink
    in: v
    field: mcp.%s.%s
    sensitivity: authoritative
    rule: r.allowed
    actor: "policy:x"
`, name, tool, arg)
}

func loaderFrom(m map[string]string) func(string) ([]byte, error) {
	return func(name string) ([]byte, error) {
		if src, ok := m[name]; ok {
			return []byte(src), nil
		}
		return nil, fmt.Errorf("recipe %q not found", name)
	}
}

func TestBuildValid(t *testing.T) {
	load := loaderFrom(map[string]string{
		"policyA": policy("policya", "write_note", "text"),
		"policyB": policy("policyb", "scale", "n"),
	})
	res := router.Build([]router.Spec{
		{Tool: "write_note", Recipe: "policyA", GateArg: "text"},
		{Tool: "scale", Recipe: "policyB", GateArg: "n"},
	}, load)
	if len(res.Router) != 2 || len(res.Errors) != 0 {
		t.Fatalf("want 2 routes, 0 errors: %d routes, %+v errors", len(res.Router), res.Errors)
	}
	if res.Router["write_note"].GateArg != "text" || res.Router["write_note"].RecipeHash == "" {
		t.Errorf("resolved route: %+v", res.Router["write_note"])
	}
}

func TestBuildFailsClosed(t *testing.T) {
	load := loaderFrom(map[string]string{
		"good":    policy("good", "write_note", "text"),
		"garbage": "this: is: not: a recipe {{{",
	})
	res := router.Build([]router.Spec{
		{Tool: "write_note", Recipe: "good", GateArg: "text"},
		{Tool: "missing_tool", Recipe: "absent", GateArg: "x"}, // loader errors
		{Tool: "bad_tool", Recipe: "garbage", GateArg: "x"},    // parse errors
	}, load)

	if len(res.Router) != 1 {
		t.Fatalf("only the valid tool routes: %d", len(res.Router))
	}
	if _, ok := res.Router["write_note"]; !ok {
		t.Error("valid tool must survive alongside bad ones")
	}
	if _, ok := res.Router["missing_tool"]; ok {
		t.Error("missing recipe must not route (fail closed)")
	}
	if _, ok := res.Router["bad_tool"]; ok {
		t.Error("invalid recipe must not route (fail closed)")
	}
	if len(res.Errors) != 2 {
		t.Errorf("both failures reported: %+v", res.Errors)
	}
}

func FuzzBuild(f *testing.F) {
	f.Add([]byte(policy("p", "t", "a")))
	f.Add([]byte(""))
	f.Add([]byte("garbage {{{ not yaml"))
	f.Fuzz(func(t *testing.T, src []byte) {
		load := func(name string) ([]byte, error) {
			if name == "r" {
				return src, nil
			}
			return nil, fmt.Errorf("no %q", name)
		}
		specs := []router.Spec{{Tool: "t", Recipe: "r", GateArg: "a"}}
		res := router.Build(specs, load)

		_, routed := res.Router["t"]
		errored := false
		for _, e := range res.Errors {
			if e.Tool == "t" {
				errored = true
			}
		}
		if routed == errored {
			t.Fatalf("exactly one of routed/errored must hold: routed=%v errored=%v", routed, errored)
		}

		_, perr := recipe.Parse(src)
		if routed {
			if perr != nil {
				t.Fatalf("routed but recipe does not parse: %v", perr)
			}
			if res.Router["t"].GateArg != "a" {
				t.Fatalf("gate arg lost: %+v", res.Router["t"])
			}
		} else if perr == nil {
			t.Fatalf("errored but recipe parses fine")
		}

		if res2 := router.Build(specs, load); !reflect.DeepEqual(res, res2) {
			t.Fatalf("nondeterministic")
		}
	})
}
