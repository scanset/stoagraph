package mcpgate

// file-kw: mcp discover admin client tools-list downstream stdio http transport quarantined

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Auth is a downstream server's client credential (Planning/28). It applies to HTTP transports only
// (a stdio subprocess authenticates via the proxy's process env). Scheme "none"/"" = no auth. The
// gate holds the credential so the agent never does (credential isolation).
type Auth struct {
	Scheme     string // none | bearer | header  (oauth is v1.1, rejected here)
	Header     string // for scheme "header": the header name
	Credential string // the resolved secret
}

// authRoundTripper injects a static credential header on every request to the downstream.
type authRoundTripper struct {
	header, value string
	next          http.RoundTripper
}

func (a authRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set(a.header, a.value)
	return a.next.RoundTrip(r)
}

// DiscoveredTool is a tool found on a downstream MCP server's tools/list.
type DiscoveredTool struct {
	Name        string
	Description string
	InputSchema string // JSON schema, marshaled
}

// DiscoverTools connects to a downstream MCP server and lists its tools — the admin
// operation that populates the config store. kind "stdio" runs `target` as a command
// and speaks MCP over its stdio; kind "http" connects to the target URL (with auth).
func DiscoverTools(ctx context.Context, kind, target string, a Auth) ([]DiscoveredTool, error) {
	t, err := transportFor(kind, target, a)
	if err != nil {
		return nil, err
	}
	return Discover(ctx, t)
}

// transportFor builds the client transport for a downstream server config. For an HTTP downstream it
// injects the configured credential (bearer/header) — FAIL CLOSED: a scheme that needs a credential
// which is empty returns an error, so the proxy never silently connects unauthenticated.
func transportFor(kind, target string, a Auth) (mcp.Transport, error) {
	switch kind {
	case "stdio":
		fields := strings.Fields(target)
		if len(fields) == 0 {
			return nil, fmt.Errorf("mcpgate: empty stdio command")
		}
		return &mcp.CommandTransport{Command: exec.Command(fields[0], fields[1:]...)}, nil
	case "http":
		if target == "" {
			return nil, fmt.Errorf("mcpgate: empty http endpoint")
		}
		t := &mcp.StreamableClientTransport{Endpoint: target}
		switch a.Scheme {
		case "", "none":
			// no auth
		case "bearer":
			if a.Credential == "" {
				return nil, fmt.Errorf("mcpgate: bearer auth configured but credential is empty (check secret / secret_env)")
			}
			t.HTTPClient = &http.Client{Transport: authRoundTripper{"Authorization", "Bearer " + a.Credential, http.DefaultTransport}}
		case "header":
			if a.Header == "" {
				return nil, fmt.Errorf("mcpgate: header auth needs a header name")
			}
			if a.Credential == "" {
				return nil, fmt.Errorf("mcpgate: header auth configured but credential is empty (check secret / secret_env)")
			}
			t.HTTPClient = &http.Client{Transport: authRoundTripper{a.Header, a.Credential, http.DefaultTransport}}
		case "oauth":
			return nil, fmt.Errorf("mcpgate: oauth downstream auth is not supported yet (v1.1)")
		default:
			return nil, fmt.Errorf("mcpgate: unknown auth scheme %q", a.Scheme)
		}
		return t, nil
	default:
		return nil, fmt.Errorf("mcpgate: unknown transport kind %q", kind)
	}
}

// Connect dials a downstream MCP server and returns a LIVE client session plus its tools
// (as *mcp.Tool, ready for NewGatingServer). The caller OWNS the session and must Close
// it. Unlike Discover (which lists then closes), the gating proxy keeps this session open
// to forward cleared calls. Fail-closed: any connect/list error returns no session.
func Connect(ctx context.Context, kind, target string, a Auth) (*mcp.ClientSession, []*mcp.Tool, error) {
	t, err := transportFor(kind, target, a)
	if err != nil {
		return nil, nil, err
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "stag-proxy", Version: "0.1"}, nil)
	sess, err := client.Connect(ctx, t, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("mcpgate: connect downstream: %w", err)
	}
	res, err := sess.ListTools(ctx, nil)
	if err != nil {
		_ = sess.Close()
		return nil, nil, fmt.Errorf("mcpgate: list downstream tools: %w", err)
	}
	return sess, res.Tools, nil
}

// Discover connects a client over t, lists the server's tools, and returns them.
// Transport-agnostic so it can be driven by an in-memory transport in tests.
func Discover(ctx context.Context, t mcp.Transport) ([]DiscoveredTool, error) {
	client := mcp.NewClient(&mcp.Implementation{Name: "stag-admin", Version: "0.1"}, nil)
	sess, err := client.Connect(ctx, t, nil)
	if err != nil {
		return nil, fmt.Errorf("mcpgate: connect: %w", err)
	}
	defer sess.Close()
	res, err := sess.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("mcpgate: list tools: %w", err)
	}
	out := make([]DiscoveredTool, 0, len(res.Tools))
	for _, tl := range res.Tools {
		schema := ""
		if tl.InputSchema != nil {
			if b, mErr := json.Marshal(tl.InputSchema); mErr == nil {
				schema = string(b)
			}
		}
		out = append(out, DiscoveredTool{Name: tl.Name, Description: tl.Description, InputSchema: schema})
	}
	return out, nil
}
