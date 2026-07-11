package claude_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/scanset/stoagraph/stoa-kernel/harness/model"
	"github.com/scanset/stoagraph/stoa-kernel/harness/model/claude"
	stag "github.com/scanset/stoagraph/stoa-kernel/stag"
)

// compile-time: a signature drift fails the build.
var _ model.Proposer = claude.Claude{}

const rh = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// msgJSON builds a canonical Messages API response body with the given text blocks.
func msgJSON(servedModel, stop string, texts ...string) string {
	blocks := make([]string, len(texts))
	for i, t := range texts {
		blocks[i] = fmt.Sprintf(`{"type":"text","text":%q}`, t)
	}
	return fmt.Sprintf(
		`{"id":"msg_1","type":"message","role":"assistant","model":%q,"content":[%s],"stop_reason":%q,"stop_sequence":null,"usage":{"input_tokens":5,"output_tokens":1}}`,
		servedModel, strings.Join(blocks, ","), stop)
}

// fakeAPI serves (status, body) at POST /v1/messages and records the last request body.
func fakeAPI(t *testing.T, status int, body string) (*httptest.Server, *[]byte) {
	t.Helper()
	var last []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		last, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &last
}

func newClaude(t *testing.T, m anthropic.Model, srv *httptest.Server) claude.Claude {
	t.Helper()
	return claude.New(m, option.WithBaseURL(srv.URL), option.WithAPIKey("test"), option.WithMaxRetries(0))
}

func TestProposeSuccess(t *testing.T) {
	srv, _ := fakeAPI(t, 200, msgJSON("claude-opus-4-8", "end_turn", "restart"))
	c := newClaude(t, "", srv)

	p, err := c.Propose(context.Background(), model.Request{Input: "act"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Value != "restart" || p.Model != "claude:claude-opus-4-8" {
		t.Errorf("proposal: %+v", p)
	}

	// determinism under a fixed transport
	p2, _ := c.Propose(context.Background(), model.Request{Input: "act"})
	if p != p2 {
		t.Errorf("nondeterministic: %+v vs %+v", p, p2)
	}

	// two text blocks concatenate in order
	srv2, _ := fakeAPI(t, 200, msgJSON("claude-opus-4-8", "end_turn", "res", "tart"))
	c2 := newClaude(t, "", srv2)
	if p3, _ := c2.Propose(context.Background(), model.Request{}); p3.Value != "restart" {
		t.Errorf("concat: %q", p3.Value)
	}
}

func TestProposeFailClosed(t *testing.T) {
	// API error -> error + zero proposal
	srvErr, _ := fakeAPI(t, 400, `{"type":"error","error":{"type":"invalid_request_error","message":"bad"}}`)
	if p, err := newClaude(t, "", srvErr).Propose(context.Background(), model.Request{Input: "x"}); err == nil || p != (model.Proposal{}) {
		t.Errorf("api error: p=%+v err=%v", p, err)
	}

	// refusal -> error + zero proposal (never empty-but-nil success)
	srvRef, _ := fakeAPI(t, 200, msgJSON("claude-opus-4-8", "refusal"))
	if p, err := newClaude(t, "", srvRef).Propose(context.Background(), model.Request{Input: "x"}); err == nil || p != (model.Proposal{}) {
		t.Errorf("refusal: p=%+v err=%v", p, err)
	}
}

func TestProposeRequestShape(t *testing.T) {
	srv, last := fakeAPI(t, 200, msgJSON("claude-opus-4-8", "end_turn", "ok"))
	c := newClaude(t, "", srv)
	if _, err := c.Propose(context.Background(), model.Request{System: "be terse", Input: "do it"}); err != nil {
		t.Fatal(err)
	}
	body := string(*last)
	if !strings.Contains(body, "do it") || !strings.Contains(body, "be terse") {
		t.Errorf("body missing input/system: %s", body)
	}
	for _, banned := range []string{"temperature", "top_p", "top_k"} {
		if strings.Contains(body, banned) {
			t.Errorf("body must not send %q (400 on Opus 4.7+): %s", banned, body)
		}
	}
	if !strings.Contains(body, "claude-opus-4-8") {
		t.Errorf("empty Model should default to opus 4.8: %s", body)
	}

	// explicit haiku model rides through
	srvH, lastH := fakeAPI(t, 200, msgJSON("claude-haiku-4-5", "end_turn", "ok"))
	if _, err := newClaude(t, anthropic.ModelClaudeHaiku4_5, srvH).Propose(context.Background(), model.Request{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(*lastH), "claude-haiku-4-5") {
		t.Errorf("haiku model not sent: %s", *lastH)
	}
}

func sampleRecipe() stag.Recipe {
	set := &stag.ReleaseRule{Kind: stag.RuleSetMembership, Set: []string{"restart", "isolate", "notify"}}
	return stag.Recipe{Steps: []stag.Step{
		{Id: "p", Kind: stag.NodePropose, Out: "action"},
		{Id: "act", Kind: stag.NodeSink, In: "action", Sensitivity: stag.SinkAuthoritative,
			Rule: set, RuleID: "actions.approved", Field: "act.args.action", Actor: "policy:remediation"},
		{Id: "log", Kind: stag.NodeSink, In: "action", Sensitivity: stag.SinkBenign, Field: "log.action"},
	}}
}

func TestClaudeDecideZeroTrust(t *testing.T) {
	ctx := context.Background()
	r := sampleRecipe()

	for _, value := range []string{"restart", "rm -rf /"} {
		srv, _ := fakeAPI(t, 200, msgJSON("claude-opus-4-8", "end_turn", value))
		c := newClaude(t, "", srv)

		dc, err := model.Decide(ctx, r, rh, c, model.Request{})
		if err != nil {
			t.Fatalf("decide(claude): %v", err)
		}
		ds, _ := model.Decide(ctx, r, rh, model.LocalStub{Name: "s", Default: value}, model.Request{})

		// verdict depends only on the value: Claude == LocalStub == direct Eval
		if !reflect.DeepEqual(dc.Result, stag.Eval(r, value, rh)) {
			t.Errorf("%q: claude decide diverged from eval", value)
		}
		if !reflect.DeepEqual(dc.Result, ds.Result) {
			t.Errorf("%q: verdict changed with the proposer (claude vs stub)", value)
		}
		// provenance carried, distinct, never authorizing
		if dc.Proposal.Model == ds.Proposal.Model {
			t.Errorf("%q: provenance should differ", value)
		}
	}
}
