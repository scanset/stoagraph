package mcpgate

// file-kw: fleet downstreams multi-server tool owner route dispatch ambiguous fail-closed

import (
	"fmt"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Downstream is one connected MCP server and the tools it exposes.
// kw: downstream name session tools
type Downstream struct {
	Name    string
	Session *mcp.ClientSession
	Tools   []*mcp.Tool
}

// Fleet is every connected downstream, addressed BY NAME.
//
// The gate fronts several tool servers at once and a cleared call must reach the right one. The ROUTE
// says which: a route is tool -> server -> recipe -> gateArg, and the server is part of the binding.
//
// The gate deliberately does NOT work out the server from the tool name. Inference would mean that
// registering an unrelated MCP server could change — or invalidate — a route you already wrote: two
// servers both happen to expose `search_code`, and a route that worked yesterday now resolves somewhere
// else, or nowhere. A policy that quietly changes when you add a server is precisely the surprise this
// product exists to eliminate. The route means the same thing tomorrow as it does today.
//
// So there is no ambiguity to resolve here, and no "owner" to guess: two servers may both expose
// `search_code` and both be routed, because each route names its own server.
// kw: fleet by-name lookup route-declares-server no-inference
type Fleet struct {
	byName map[string]Downstream           // server name -> the connected session
	tools  map[string]map[string]*mcp.Tool // server name -> tool name -> declaration
}

// NewFleet indexes the connected downstreams by NAME.
func NewFleet(downs []Downstream) Fleet {
	f := Fleet{byName: map[string]Downstream{}, tools: map[string]map[string]*mcp.Tool{}}
	for _, d := range downs {
		f.byName[d.Name] = d
		m := map[string]*mcp.Tool{}
		for _, t := range d.Tools {
			m[t.Name] = t
		}
		f.tools[d.Name] = m
	}
	return f
}

// Lookup resolves a ROUTE (server + tool) to the connected server and the tool's declaration. It fails
// when the named server is not connected, or when that server does not expose that tool — both of which
// are configuration errors the operator must be told about, never guessed around.
// kw: lookup route server tool declaration fail-closed
func (f Fleet) Lookup(server, tool string) (Downstream, *mcp.Tool, error) {
	d, ok := f.byName[server]
	if !ok {
		return Downstream{}, nil, fmt.Errorf("no connected MCP server named %q", server)
	}
	t, ok := f.tools[server][tool]
	if !ok {
		return Downstream{}, nil, fmt.Errorf("server %q does not expose a tool named %q", server, tool)
	}
	return d, t, nil
}

// Has reports whether a server is connected.
func (f Fleet) Has(server string) bool { _, ok := f.byName[server]; return ok }

// Servers names the connected downstreams.
func (f Fleet) Servers() []string {
	out := make([]string, 0, len(f.byName))
	for name := range f.byName {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}
