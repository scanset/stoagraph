// Command example-pii is the pii-demo downstream MCP server — the containment story, in one binary.
//
//	fetch_user_profile(user_id)              INTERNAL read. Returns a profile that INCLUDES an SSN.
//	send_external_reply(ticket_id, body)     EXTERNAL egress. Whatever `body` holds leaves the boundary.
//
// The agent is *supposed* to read the SSN — a support agent needs the record. What it must never do is
// let that value cross the egress boundary. The policy does not scan for SSNs, or for anything else:
// `send_external_reply` may only carry one of four approved TEMPLATE IDs. So the SSN cannot cross
// because no free-form value can cross. **Containment is structural, not content-based** — which is why
// a jailbroken, prompt-injected, or simply confused model cannot defeat it. It can propose the
// exfiltration all day; the gate will not release it.
//
// It speaks stdio (an agent host spawns it) or streamable HTTP (-http, so a CONTAINERISED gate can
// reach it over the network — a distroless gate cannot spawn a python subprocess, and we would rather
// keep the gate minimal than bake an interpreter into it).
package main

// file-kw: example downstream pii containment mcp server stdio streamable-http egress structural-not-scanning

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The "internal database". The SSN is here ON PURPOSE: the demo is only meaningful if the sensitive
// value is genuinely reachable by the agent.
var users = map[string]map[string]string{
	"123": {"name": "Alice", "ssn": "000-12-3456", "status": "active"},
	"456": {"name": "Bob", "ssn": "000-98-7654", "status": "locked"},
}

func main() {
	httpAddr := flag.String("http", "", "serve streamable HTTP on this address (empty = stdio)")
	flag.Parse()
	log.SetOutput(os.Stderr) // stdio carries the protocol on stdout

	srv := mcp.NewServer(&mcp.Implementation{Name: "pii-demo", Version: "0.1.0"}, nil)

	srv.AddTool(&mcp.Tool{
		Name:        "fetch_user_profile",
		Description: "Internal database lookup. Returns a user profile INCLUDING sensitive fields (ssn).",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"user_id": map[string]any{"type": "string"}},
			"required":   []string{"user_id"},
		},
	}, func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var a struct {
			UserID string `json:"user_id"`
		}
		_ = json.Unmarshal(req.Params.Arguments, &a)
		u, ok := users[a.UserID]
		if !ok {
			return text(fmt.Sprintf("no such user %q", a.UserID), true), nil
		}
		b, _ := json.Marshal(u)
		return text(string(b), false), nil
	})

	srv.AddTool(&mcp.Tool{
		Name:        "send_external_reply",
		Description: "Send a reply to the customer (EXTERNAL egress). message_body leaves the trust boundary.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"ticket_id":    map[string]any{"type": "string"},
				"message_body": map[string]any{"type": "string"},
			},
			"required": []string{"ticket_id", "message_body"},
		},
	}, func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// NOTE: this handler does NO checking. It cannot be reached with an unapproved body, because
		// the gate never forwards one. That is the point: the tool server is dumb, and safety does not
		// depend on it being careful.
		var a struct {
			TicketID    string `json:"ticket_id"`
			MessageBody string `json:"message_body"`
		}
		_ = json.Unmarshal(req.Params.Arguments, &a)
		return text(fmt.Sprintf("sent to customer (ticket %s): %s", a.TicketID, a.MessageBody), false), nil
	})

	ctx := context.Background()
	if *httpAddr == "" {
		log.Printf("pii-demo: serving MCP over stdio")
		if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
			log.Fatal(err)
		}
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"ok":true}`)
	})
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil))

	log.Printf("pii-demo: serving MCP over streamable HTTP on %s/mcp", *httpAddr)
	log.Fatal(http.ListenAndServe(*httpAddr, mux))
}

func text(s string, isErr bool) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: isErr, Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}
