package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/scanset/stoagraph/stoa-kernel/stag/recipestore"
	"github.com/scanset/stoagraph/stoa-kernel/stag/store"
)

const internalRecipe = `recipe: internal_lookup_policy
version: 1
steps:
  - id: propose_uid
    kind: propose
    out: uid
  - id: read
    kind: sink
    in: uid
    field: internal.user_lookup
    sensitivity: benign
`

const externalRecipe = `recipe: external_reply_policy
version: 1
rules:
  reply.templates:
    kind: set_membership
    set: ["tmpl:account_unlocked", "tmpl:looking_into_it"]
steps:
  - id: propose_body
    kind: propose
    out: body
  - id: send
    kind: sink
    in: body
    field: outbound.email.body
    sensitivity: authoritative
    rule: reply.templates
    actor: "policy:outbound_comms"
`

func textOf(r *mcp.CallToolResult) string {
	for _, c := range r.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// End-to-end: an MCP client (the "agent") connects to a spawned stag-proxy, which gates
// each call and forwards cleared ones to the real pii-demo downstream. Proves the whole
// wiring: store config -> gate -> downstream connect -> gated MCP over stdio.
func TestStagProxyE2E(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not available")
	}
	// Relative to the repo root (this test runs in cmd/stag-proxy). An absolute path here used to
	// pin the test to one machine — it would silently t.Skip everywhere else, including CI.
	serverPy, err := filepath.Abs(filepath.Join("..", "..", "..", "examples", "pii-demo", "server.py"))
	if err != nil {
		t.Skip("cannot resolve the pii-demo server path")
	}
	if _, err := os.Stat(serverPy); err != nil {
		t.Skipf("pii-demo server not found at %s", serverPy)
	}

	// build the stag-proxy binary
	bin := filepath.Join(t.TempDir(), "stag-proxy")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build stag-proxy: %v\n%s", err, out)
	}

	// temp config: register the pii-demo downstream + routes + recipes
	dir := t.TempDir()
	storePath := filepath.Join(dir, "config.db")
	recipesDir := filepath.Join(dir, "recipes")
	logPath := filepath.Join(dir, "log.jsonl")
	ctx := context.Background()
	st, err := store.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	must := func(e error) {
		if e != nil {
			t.Fatal(e)
		}
	}
	must(st.PutMCPServer(ctx, store.MCPServer{Name: "pii-demo", Transport: "stdio", Target: py + " " + serverPy, Enabled: true}))
	must(st.PutRoute(ctx, store.Route{Tool: "fetch_user_profile", Server: "pii-demo", Recipe: "internal_lookup_policy", GateArg: "user_id"}))
	must(st.PutRoute(ctx, store.Route{Tool: "send_external_reply", Server: "pii-demo", Recipe: "external_reply_policy", GateArg: "message_body"}))
	st.Close()
	rs := recipestore.Store{Dir: recipesDir}
	if _, err := rs.Save([]byte(internalRecipe)); err != nil {
		t.Fatal(err)
	}
	if _, err := rs.Save([]byte(externalRecipe)); err != nil {
		t.Fatal(err)
	}

	// spawn stag-proxy and connect an MCP client (the "agent") to it
	cmd := exec.Command(bin, "-store", storePath, "-recipes-dir", recipesDir, "-log", logPath, "-downstream", "pii-demo")
	agent := mcp.NewClient(&mcp.Implementation{Name: "test-agent", Version: "0"}, nil)
	sess, err := agent.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("connect to stag-proxy: %v", err)
	}
	defer sess.Close()

	lt, err := sess.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(lt.Tools) != 2 {
		t.Fatalf("gated tool surface = %d, want 2 (the downstream tools)", len(lt.Tools))
	}

	// ALLOW: internal read is cleared, forwarded, downstream returns the profile
	r1, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "fetch_user_profile", Arguments: map[string]any{"user_id": "123"}})
	if err != nil {
		t.Fatal(err)
	}
	if r1.IsError {
		t.Errorf("fetch_user_profile should be ALLOWED, got error: %s", textOf(r1))
	}

	// DENY: PII in the outbound body — gate blocks it, downstream NEVER called
	r2, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "send_external_reply", Arguments: map[string]any{"ticket_id": "123", "message_body": "Here is your data: 000-12-3456"}})
	if err != nil {
		t.Fatal(err)
	}
	if !r2.IsError {
		t.Errorf("PII send must be DENIED by the gate, but it was forwarded: %s", textOf(r2))
	}

	// ALLOW: an approved template crosses
	r3, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "send_external_reply", Arguments: map[string]any{"ticket_id": "123", "message_body": "tmpl:account_unlocked"}})
	if err != nil {
		t.Fatal(err)
	}
	if r3.IsError {
		t.Errorf("approved template should be ALLOWED, got error: %s", textOf(r3))
	}
}
