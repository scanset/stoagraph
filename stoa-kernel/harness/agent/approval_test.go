package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestEscalationID(t *testing.T) {
	// an approval-gated escalate carries the id in _meta.stag
	esc := &mcp.CallToolResult{Meta: mcp.Meta{"stag": map[string]any{"verdict": "escalate", "approvalId": "abc123"}}}
	if id, ok := escalationID(esc); !ok || id != "abc123" {
		t.Errorf("escalate result: got (%q,%v), want (abc123,true)", id, ok)
	}
	// a plain deny (no approvalId) is NOT an approval wait
	deny := &mcp.CallToolResult{Meta: mcp.Meta{"stag": map[string]any{"verdict": "deny"}}}
	if _, ok := escalationID(deny); ok {
		t.Error("deny without approvalId must not trigger an approval wait")
	}
	// an allowed call (no meta) is not an escalation
	if _, ok := escalationID(&mcp.CallToolResult{}); ok {
		t.Error("a result with no gate meta must not be an escalation")
	}
	// an escalate WITHOUT an approvalId (e.g. a non-approval escalate gate) must not wait
	bare := &mcp.CallToolResult{Meta: mcp.Meta{"stag": map[string]any{"verdict": "escalate"}}}
	if _, ok := escalationID(bare); ok {
		t.Error("escalate without an approvalId must not trigger a wait")
	}
}

func TestAwaitApproved(t *testing.T) {
	// stag-serve stub: pending for the first 2 polls, then approved with a token.
	var polls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&polls, 1)
		w.Header().Set("Content-Type", "application/json")
		if n < 3 {
			_, _ = w.Write([]byte(`{"status":"pending"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"approved","token":"SIGNED-TOK"}`))
	}))
	defer srv.Close()

	appr := &ApprovalConfig{BaseURL: srv.URL, Poll: 5 * time.Millisecond, Timeout: 2 * time.Second, HTTP: srv.Client()}
	token, status, err := appr.await(context.Background(), "id1")
	if err != nil {
		t.Fatalf("await error: %v", err)
	}
	if status != "approved" || token != "SIGNED-TOK" {
		t.Fatalf("await = (%q,%q), want (SIGNED-TOK, approved)", token, status)
	}
	if polls < 3 {
		t.Errorf("expected to poll until approved, got %d polls", polls)
	}
}

func TestAwaitDeniedAndTimeout(t *testing.T) {
	denySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"denied"}`))
	}))
	defer denySrv.Close()
	appr := &ApprovalConfig{BaseURL: denySrv.URL, Poll: 5 * time.Millisecond, Timeout: time.Second, HTTP: denySrv.Client()}
	if _, status, err := appr.await(context.Background(), "id"); err != nil || status != "denied" {
		t.Fatalf("denied: got (%q,%v), want denied", status, err)
	}

	pendSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"pending"}`))
	}))
	defer pendSrv.Close()
	appr2 := &ApprovalConfig{BaseURL: pendSrv.URL, Poll: 5 * time.Millisecond, Timeout: 30 * time.Millisecond, HTTP: pendSrv.Client()}
	if _, status, err := appr2.await(context.Background(), "id"); err != nil || status != "timeout" {
		t.Fatalf("timeout: got (%q,%v), want timeout", status, err)
	}
}
