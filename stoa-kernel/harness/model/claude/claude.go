// Package claude is the Claude proposer adapter: a model.Proposer that calls
// the Anthropic Messages API and returns the completion as an untrusted
// proposal. The anthropic SDK dependency is quarantined here.
package claude

// file-kw: claude proposer adapter anthropic messages api untrusted transport quarantine fail-closed

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/scanset/stoagraph/stoa-kernel/harness/model"
)

// kw: default model opus 4.8 haiku cheap proposer
const DefaultModel = anthropic.ModelClaudeOpus4_8

const defaultMaxTokens int64 = 1024

// kw: claude proposer strategy model maxtokens client
type Claude struct {
	Model     anthropic.Model
	MaxTokens int64
	client    anthropic.Client
}

// kw: new claude inject options client
func New(m anthropic.Model, opts ...option.RequestOption) Claude {
	return Claude{Model: m, client: anthropic.NewClient(opts...)}
}

// kw: propose call messages api untrusted fail-closed no temperature
func (c Claude) Propose(ctx context.Context, req model.Request) (model.Proposal, error) {
	m := c.Model
	if m == "" {
		m = DefaultModel
	}
	maxTok := c.MaxTokens
	if maxTok <= 0 {
		maxTok = defaultMaxTokens
	}
	params := anthropic.MessageNewParams{
		Model:     m,
		MaxTokens: maxTok,
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(req.Input))},
		// temperature/top_p/top_k are never set (400 on Opus 4.7+); steer by prompt.
	}
	if req.System != "" {
		params.System = []anthropic.TextBlockParam{{Text: req.System}}
	}
	resp, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return model.Proposal{}, fmt.Errorf("claude: %w", err) // fail closed (inv 8)
	}
	if resp.StopReason == anthropic.StopReasonRefusal {
		return model.Proposal{}, fmt.Errorf("claude: model declined (refusal)") // fail closed
	}
	var b strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			b.WriteString(t.Text)
		}
	}
	// provenance: the model that actually served; Value is stamped Untrusted by Eval.
	return model.Proposal{Value: b.String(), Model: "claude:" + string(resp.Model)}, nil
}
