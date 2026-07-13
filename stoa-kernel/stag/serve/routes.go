package serve

// file-kw: route endpoints tool recipe binding list put delete resolution status multi-tool

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"

	"github.com/scanset/stoagraph/stoa-kernel/stag/router"
	"github.com/scanset/stoagraph/stoa-kernel/stag/store"
)

// kw: route view tool recipe gatearg valid error resolution
type RouteView struct {
	Tool    string `json:"tool"`
	Server  string `json:"server"` // the MCP server this tool is dispatched to
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
		specs = append(specs, router.Spec{Tool: rt.Tool, Server: rt.Server, Recipe: rt.Recipe, GateArg: rt.GateArg})
	}
	resolved := router.Build(specs, s.Recipes.Get)
	errBy := map[string]string{}
	for _, e := range resolved.Errors {
		errBy[e.Tool] = e.Err
	}
	out := make([]RouteView, 0, len(routes))
	for _, rt := range routes {
		_, ok := resolved.Router[rt.Tool]
		out = append(out, RouteView{Tool: rt.Tool, Server: rt.Server, Recipe: rt.Recipe, GateArg: rt.GateArg, Valid: ok, Error: errBy[rt.Tool]})
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /api/routes — {tool, server, recipe, gateArg}; upserts the binding.
//
// The SERVER is part of the route, and required. The gate must never work out which downstream a tool
// belongs to: if it did, registering an unrelated MCP server that happens to expose the same tool name
// could change — or invalidate — a route you already wrote. A route means the same thing tomorrow.
func (s *Server) handleRoutePut(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		writeJSON(w, http.StatusNotImplemented, errObj("no config store"))
		return
	}
	body, _ := io.ReadAll(r.Body)
	var req struct {
		Tool    string `json:"tool"`
		Server  string `json:"server"`
		Recipe  string `json:"recipe"`
		GateArg string `json:"gateArg"`
	}
	if json.Unmarshal(body, &req) != nil || req.Tool == "" || req.Recipe == "" || req.GateArg == "" {
		writeJSON(w, http.StatusBadRequest, errObj("route needs a tool, server, recipe, and gateArg"))
		return
	}
	if req.Server == "" {
		writeJSON(w, http.StatusBadRequest, errObj("route needs a `server`: which MCP server serves this tool"))
		return
	}
	// The named server must exist AND expose this tool. Catching it here means the operator learns at
	// authoring time, not when an agent's call mysteriously fails.
	sv, gerr := s.Store.GetMCPServer(r.Context(), req.Server)
	if gerr != nil {
		writeJSON(w, http.StatusBadRequest, errObj("unknown MCP server: "+req.Server))
		return
	}
	if len(sv.Tools) > 0 && !slices.ContainsFunc(sv.Tools, func(t store.MCPTool) bool { return t.Name == req.Tool }) {
		writeJSON(w, http.StatusBadRequest, errObj(
			"server "+req.Server+" does not expose a tool named "+req.Tool))
		return
	}
	if err := s.Store.PutRoute(r.Context(), store.Route{Tool: req.Tool, Server: req.Server, Recipe: req.Recipe, GateArg: req.GateArg}); err != nil {
		writeJSON(w, http.StatusInternalServerError, errObj(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tool": req.Tool, "server": req.Server, "recipe": req.Recipe, "gateArg": req.GateArg})
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
