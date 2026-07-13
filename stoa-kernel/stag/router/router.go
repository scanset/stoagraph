// Package router resolves the persisted route table (Planning/18) into a live
// proxy.Router — the step that makes the gate MULTI-TOOL from saved bindings. A
// stored route binds a tool to a recipe BY NAME + a gated arg; Build loads and
// parses each recipe into the kernel form. Fail closed: a route whose recipe is
// missing or invalid produces NO router entry (and an error) — the tool is left
// unrouted, which the gate denies (U22). The router never holds a broken recipe.
package router

// file-kw: route resolve build proxy router recipe-by-name fail-closed multi-tool gate binding

import (
	"github.com/scanset/stoagraph/stoa-kernel/stag/proxy"
	"github.com/scanset/stoagraph/stoa-kernel/stag/recipe"
)

// kw: spec tool recipe-name gate-arg (a stored binding)
type Spec struct {
	Tool    string
	Server  string // the MCP server this tool is dispatched to. Declared, never inferred.
	Recipe  string
	GateArg string
}

// kw: route error tool recipe reason unresolved
type RouteError struct {
	Tool   string
	Recipe string
	Err    string
}

// kw: resolved router errors
type Resolved struct {
	Router proxy.Router
	Errors []RouteError
}

// kw: build resolve specs load parse fail-closed
func Build(specs []Spec, loadRecipe func(name string) ([]byte, error)) Resolved {
	res := Resolved{Router: proxy.Router{}}
	for _, sp := range specs {
		src, err := loadRecipe(sp.Recipe)
		if err != nil {
			res.Errors = append(res.Errors, RouteError{Tool: sp.Tool, Recipe: sp.Recipe, Err: err.Error()})
			continue
		}
		// Compose: a routed recipe may inline sub-recipes (goto_recipe); resolve them via
		// the same loader. A plain recipe composes to itself. The gate binds the COMPOSED
		// hash, so the audit proves exactly the policy that ran.
		p, _, err := recipe.Compose(src, loadRecipe)
		if err != nil {
			res.Errors = append(res.Errors, RouteError{Tool: sp.Tool, Recipe: sp.Recipe, Err: err.Error()})
			continue // fail closed: no entry -> tool unrouted -> gate denies
		}
		res.Router[sp.Tool] = proxy.Route{Recipe: p.Recipe, RecipeHash: p.SemanticHash, GateArg: sp.GateArg, RecipeName: sp.Recipe, Server: sp.Server}
	}
	return res
}
