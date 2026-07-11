// Command harness is the minimal orchestrator: it connects to a model (Claude) and routes
// the model's proposed tool calls through the stag-proxy MCP server, which gates each one.
// The model is the untrusted proposer; stag decides what is allowed. This is the seed
// of the event_harness agent loop (Planning/22/23) — event ingress + recipe dispatch come
// later; for now it takes a system + input prompt and runs one governed agent turn-loop.
//
// The harness holds the model key (env var); stag never does. It speaks MCP to
// stag-proxy as a client — every tool the model calls is gated before it reaches downstream.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	modelID := flag.String("model", "claude-haiku-4-5", "Claude model id (the proposer)")
	keyEnv := flag.String("key-env", "ANTHROPIC_API_KEY", "env var holding the API key")
	proxyCmd := flag.String("proxy", "", "command that runs stag-proxy (the gating MCP server), e.g. '/tmp/stag-proxy -downstream pii-demo'")
	system := flag.String("system", "You are a support agent. Use the available tools to help. Only send approved template ids in replies.", "system prompt (trusted)")
	input := flag.String("input", "", "the task / event (untrusted)")
	maxTurns := flag.Int("max-turns", 6, "max model<->tool round trips")
	flag.Parse()

	if *proxyCmd == "" || *input == "" {
		log.Fatal("need -proxy (the stag-proxy command) and -input")
	}
	key := os.Getenv(*keyEnv)
	if key == "" {
		log.Fatalf("no API key in $%s", *keyEnv)
	}
	ctx := context.Background()

	// connect to stag-proxy as an MCP client — the model's tools come from here, GATED.
	fields := strings.Fields(*proxyCmd)
	client := mcp.NewClient(&mcp.Implementation{Name: "event-harness", Version: "0.1"}, nil)
	sess, err := client.Connect(ctx, &mcp.CommandTransport{Command: exec.Command(fields[0], fields[1:]...)}, nil)
	if err != nil {
		log.Fatalf("connect to stag-proxy: %v", err)
	}
	defer sess.Close()

	lt, err := sess.ListTools(ctx, nil)
	if err != nil {
		log.Fatalf("list gated tools: %v", err)
	}
	tools := make([]anthropic.ToolUnionParam, 0, len(lt.Tools))
	for _, t := range lt.Tools {
		tools = append(tools, anthropic.ToolUnionParam{OfTool: &anthropic.ToolParam{
			Name:        t.Name,
			Description: anthropic.String(t.Description),
			InputSchema: toSchema(t.InputSchema),
		}})
	}
	fmt.Printf("→ %d gated tool(s) from stag-proxy: %s\n\n", len(tools), toolNames(lt.Tools))

	// the tool-use loop: the model proposes tool calls; each is routed through stag-proxy.
	llm := anthropic.NewClient(option.WithAPIKey(key))
	messages := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(*input))}

	for turn := 0; turn < *maxTurns; turn++ {
		resp, err := llm.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(*modelID),
			MaxTokens: 1024,
			System:    []anthropic.TextBlockParam{{Text: *system}},
			Messages:  messages,
			Tools:     tools,
		})
		if err != nil {
			log.Fatalf("model: %v", err)
		}
		messages = append(messages, resp.ToParam())

		var results []anthropic.ContentBlockParamUnion
		for _, block := range resp.Content {
			switch b := block.AsAny().(type) {
			case anthropic.TextBlock:
				if strings.TrimSpace(b.Text) != "" {
					fmt.Printf("🗣  model: %s\n", b.Text)
				}
			case anthropic.ToolUseBlock:
				// route the proposed call THROUGH stag-proxy — it gates, then forwards or refuses.
				var args map[string]any
				_ = json.Unmarshal(b.Input, &args)
				fmt.Printf("🔧 propose %s(%s)\n", b.Name, compact(b.Input))
				out, isErr := callGated(ctx, sess, b.Name, args)
				verdict := "✅ allowed"
				if isErr {
					verdict = "⛔ gate refused"
				}
				fmt.Printf("   %s → %s\n", verdict, out)
				results = append(results, anthropic.NewToolResultBlock(b.ID, out, isErr))
			}
		}
		if len(results) == 0 {
			fmt.Println("\n✔ done (model stopped proposing tools)")
			return
		}
		messages = append(messages, anthropic.NewUserMessage(results...))
	}
	fmt.Printf("\n… stopped after %d turns\n", *maxTurns)
}

// callGated sends one tool call through stag-proxy and returns its text + whether the gate
// (or the downstream) reported an error. A gate denial comes back as IsError, so the model
// sees the refusal and can adapt — but the call never reached the real tool.
func callGated(ctx context.Context, sess *mcp.ClientSession, name string, args map[string]any) (string, bool) {
	res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		return fmt.Sprintf("transport error: %v", err), true
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String(), res.IsError
}

func toSchema(s any) anthropic.ToolInputSchemaParam {
	out := anthropic.ToolInputSchemaParam{}
	if s == nil {
		return out
	}
	b, err := json.Marshal(s)
	if err != nil {
		return out
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return out
	}
	out.Properties = m["properties"]
	if req, ok := m["required"].([]any); ok {
		for _, r := range req {
			out.Required = append(out.Required, fmt.Sprint(r))
		}
	}
	return out
}

func toolNames(tools []*mcp.Tool) string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Name
	}
	return strings.Join(names, ", ")
}

func compact(raw json.RawMessage) string {
	var buf bytes.Buffer
	if json.Compact(&buf, raw) != nil {
		return string(raw)
	}
	return buf.String()
}
