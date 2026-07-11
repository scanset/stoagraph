package serve

// file-kw: route endpoints tool recipe binding list put delete resolution status multi-tool

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/scanset/stoagraph/stoa-kernel/stag/router"
	"github.com/scanset/stoagraph/stoa-kernel/stag/store"
)

// kw: route view tool recipe gatearg valid error resolution
type RouteView struct {
	Tool    string `json:"tool"`
	Recipe  string `json:"recipe"`
	GateArg string `json:"gateArg"`
	Valid   bool   `json:"valid"`           // does the bound recipe resolve (load + parse)?
	Error   string `json:"error,omitempty"` // why not, if invalid
}

// GET /api/routes — the tool→recipe bindings with their resolution status (a route
// whose recipe is missing/invalid is shown invalid; that tool is denied by default).
func (s *Server) handleRouteList(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		writeJSON(w, http.StatusOK, []RouteView{})
		return
	}
	routes, err := s.Store.ListRoutes(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errObj(err.Error()))
		return
	}
	specs := make([]router.Spec, 0, len(routes))
	for _, rt := range routes {
		specs = append(specs, router.Spec{Tool: rt.Tool, Recipe: rt.Recipe, GateArg: rt.GateArg})
	}
	resolved := router.Build(specs, s.Recipes.Get)
	errBy := map[string]string{}
	for _, e := range resolved.Errors {
		errBy[e.Tool] = e.Err
	}
	out := make([]RouteView, 0, len(routes))
	for _, rt := range routes {
		_, ok := resolved.Router[rt.Tool]
		out = append(out, RouteView{Tool: rt.Tool, Recipe: rt.Recipe, GateArg: rt.GateArg, Valid: ok, Error: errBy[rt.Tool]})
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /api/routes — {tool, recipe, gateArg}; upserts the binding.
func (s *Server) handleRoutePut(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		writeJSON(w, http.StatusNotImplemented, errObj("no config store"))
		return
	}
	body, _ := io.ReadAll(r.Body)
	var req struct {
		Tool    string `json:"tool"`
		Recipe  string `json:"recipe"`
		GateArg string `json:"gateArg"`
	}
	if json.Unmarshal(body, &req) != nil || req.Tool == "" || req.Recipe == "" || req.GateArg == "" {
		writeJSON(w, http.StatusBadRequest, errObj("route needs a tool, recipe, and gateArg"))
		return
	}
	if err := s.Store.PutRoute(r.Context(), store.Route{Tool: req.Tool, Recipe: req.Recipe, GateArg: req.GateArg}); err != nil {
		writeJSON(w, http.StatusInternalServerError, errObj(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tool": req.Tool})
}

// DELETE /api/routes/{tool}
func (s *Server) handleRouteDelete(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		writeJSON(w, http.StatusNotImplemented, errObj("no config store"))
		return
	}
	if err := s.Store.DeleteRoute(r.Context(), r.PathValue("tool")); err != nil {
		writeJSON(w, http.StatusInternalServerError, errObj(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": r.PathValue("tool")})
}
