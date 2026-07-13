package mcpgate_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/scanset/stoagraph/stoa-kernel/stag/provider"
	"github.com/scanset/stoagraph/stoa-kernel/stag/proxy"
	"github.com/scanset/stoagraph/stoa-kernel/stag/proxy/mcpgate"
)

// TestReadChannelServesLabeledContext drives the READ channel end-to-end over real MCP transports:
// agent -> [stag gating server] -> context provider (an httptest KB). It asserts the three READ-channel
// contracts of Planning/30: the provider is advertised as a resource TEMPLATE, a resources/read passes
// the ?q query through and returns the body stamped UNTRUSTED, and the crossing is RECORDED.
func TestReadChannelServesLabeledContext(t *testing.T) {
	ctx := context.Background()

	// a downstream "KB": echoes the query it was asked, so we can prove ?q reached the provider.
	kb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("fact for: " + r.URL.Query().Get("q")))
	}))
	defer kb.Close()

	// the recorded READ crossings (reads happen in the server goroutine; guard the capture).
	var mu sync.Mutex
	var recorded []provider.ReadEvent
	record := func(_ context.Context, ev provider.ReadEvent) {
		mu.Lock()
		defer mu.Unlock()
		recorded = append(recorded, ev)
	}

	read := mcpgate.ReadChannel{
		Providers: []provider.ContextProvider{provider.HTTP{ProviderName: "kb", URL: kb.URL}},
		Record:    record,
	}
	// READ-only server: no tools, so the (nil) downstream is never touched.
	gatingSrv := mcpgate.NewGatingServer(proxy.Gate{Routes: proxy.Router{}}, mcpgate.Fleet{}, read)

	aClientT, aServerT := mcp.NewInMemoryTransports()
	gatingSess, err := gatingSrv.Connect(ctx, aServerT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer gatingSess.Close()
	agent := mcp.NewClient(&mcp.Implementation{Name: "agent", Version: "0"}, nil)
	agentSess, err := agent.Connect(ctx, aClientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer agentSess.Close()

	// 1. the provider is advertised as a resource TEMPLATE (queryable), not a fixed resource.
	tmpls, err := agentSess.ListResourceTemplates(ctx, &mcp.ListResourceTemplatesParams{})
	if err != nil {
		t.Fatal(err)
	}
	const wantURI = "stag://context/kb{?q}"
	found := false
	for _, tp := range tmpls.ResourceTemplates {
		if tp.URITemplate == wantURI {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a resource template %q; got %+v", wantURI, tmpls.ResourceTemplates)
	}

	// 2. a read passes ?q through and returns the body stamped UNTRUSTED at origin.
	res, err := agentSess.ReadResource(ctx, &mcp.ReadResourceParams{URI: "stag://context/kb?q=needle"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("expected one content item, got %d", len(res.Contents))
	}
	got := res.Contents[0].Text
	if !strings.Contains(got, "fact for: needle") {
		t.Errorf("the ?q query must reach the provider; content = %q", got)
	}
	if !strings.Contains(got, "untrusted context") || !strings.Contains(got, "NOT instructions") {
		t.Errorf("content must be labeled untrusted at origin; content = %q", got)
	}

	// 3. the crossing was RECORDED (label+record).
	mu.Lock()
	defer mu.Unlock()
	if len(recorded) != 1 {
		t.Fatalf("expected one recorded read, got %d", len(recorded))
	}
	ev := recorded[0]
	if ev.Provider != "kb" || ev.Query != "needle" || ev.Items != 1 {
		t.Errorf("read event = %+v; want provider=kb query=needle items=1", ev)
	}
}

// TestReadChannelEmptyIsHonestNotAnError asserts a read is NEVER denied: even a provider that fails
// yields a non-error, non-nil result (label+record, not allow/deny) with the failure recorded.
func TestReadChannelEmptyIsHonestNotAnError(t *testing.T) {
	ctx := context.Background()

	var mu sync.Mutex
	var recorded []provider.ReadEvent
	read := mcpgate.ReadChannel{
		// an unreachable URL => the provider errors => Gather is fail-open => zero items.
		Providers: []provider.ContextProvider{provider.HTTP{ProviderName: "down", URL: "http://127.0.0.1:0/nope"}},
		Record: func(_ context.Context, ev provider.ReadEvent) {
			mu.Lock()
			defer mu.Unlock()
			recorded = append(recorded, ev)
		},
	}
	gatingSrv := mcpgate.NewGatingServer(proxy.Gate{Routes: proxy.Router{}}, mcpgate.Fleet{}, read)

	aClientT, aServerT := mcp.NewInMemoryTransports()
	gatingSess, err := gatingSrv.Connect(ctx, aServerT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer gatingSess.Close()
	agent := mcp.NewClient(&mcp.Implementation{Name: "agent", Version: "0"}, nil)
	agentSess, err := agent.Connect(ctx, aClientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer agentSess.Close()

	res, err := agentSess.ReadResource(ctx, &mcp.ReadResourceParams{URI: "stag://context/down?q=x"})
	if err != nil {
		t.Fatalf("a failing read must NOT be an error (reads are label+record, never deny): %v", err)
	}
	if len(res.Contents) == 0 {
		t.Fatal("expected an honest empty-read marker content, got none")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(recorded) != 1 || recorded[0].Items != 0 || len(recorded[0].Errors) == 0 {
		t.Errorf("a failed read must be recorded with items=0 and an error; got %+v", recorded)
	}
}
