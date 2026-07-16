// Command stag-probe is a SCRIPTED (no-model) MCP client for the gate — the model-independent half of
// the demo matrix. It connects to a stag-proxy over stdio, then runs a tiny line-based script of gated
// tool calls and human approvals, printing tools/list and the verdict of every call. Because there is no
// model, the results are a property of the GATE, provable deterministically.
//
// Script directives (stdin), one per line ('#' comments and blank lines ignored):
//
//	list                         — print the advertised tool surface (tools/list)
//	call  <tool> <json-args>     — issue a gated call; prints allow / deny / escalate (+ approvalId)
//	approve                      — as the HUMAN, approve the last escalation (POST -serve .../approve);
//	                               stores the returned signed token as $TOKEN
//	note  <text>                 — echo a narration line
//
// In <json-args>, the literal $TOKEN is substituted with the last approved release token.
package main

// file-kw: cli scripted mcp client probe gated tools-list verdict approve token one-time deterministic

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	proxy := flag.String("proxy", "", "command to run the stag-proxy gating server over stdio, e.g. 'bin/stag-proxy -downstream ops ...'")
	serve := flag.String("serve", "http://localhost:8080", "stag-serve base URL (for the human-approve step)")
	flag.Parse()
	if *proxy == "" {
		fatal("need -proxy '<stag-proxy stdio command>'")
	}
	ctx := context.Background()

	fields := strings.Fields(*proxy)
	sess, err := mcp.NewClient(&mcp.Implementation{Name: "stag-probe", Version: "0.1"}, nil).
		Connect(ctx, &mcp.CommandTransport{Command: exec.Command(fields[0], fields[1:]...)}, nil)
	if err != nil {
		fatal("connect to stag-proxy: %v", err)
	}
	defer sess.Close()

	var token, lastApprovalID string
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		verb, rest, _ := strings.Cut(line, " ")
		switch verb {
		case "note":
			fmt.Printf("\n— %s\n", rest)

		case "list":
			lt, err := sess.ListTools(ctx, nil)
			if err != nil {
				fatal("tools/list: %v", err)
			}
			names := make([]string, len(lt.Tools))
			for i, t := range lt.Tools {
				names[i] = t.Name
			}
			fmt.Printf("  tools/list (the advertised surface): %v\n", names)

		case "call":
			tool, argsJSON, _ := strings.Cut(rest, " ")
			argsJSON = strings.ReplaceAll(strings.TrimSpace(argsJSON), "$TOKEN", token)
			var args map[string]any
			if argsJSON != "" {
				if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
					fatal("call %s: bad json args %q: %v", tool, argsJSON, err)
				}
			}
			verdict, approvalID := gatedCall(ctx, sess, tool, args)
			if approvalID != "" {
				lastApprovalID = approvalID
			}
			shown := argsJSON
			if token != "" {
				shown = strings.ReplaceAll(shown, token, "<approved-token>")
			}
			fmt.Printf("  call %-16s %-40s -> %s\n", tool, shown, verdict)

		case "approve":
			if lastApprovalID == "" {
				fatal("approve: no escalation to approve yet")
			}
			tok, err := humanApprove(*serve, lastApprovalID)
			if err != nil {
				fatal("approve %s: %v", lastApprovalID, err)
			}
			token = tok
			fmt.Printf("  approve %s -> HUMAN minted a signed release (ed25519, bound to this action)\n", lastApprovalID)

		default:
			fatal("unknown directive %q", verb)
		}
	}
	if err := sc.Err(); err != nil {
		fatal("read script: %v", err)
	}
}

// gatedCall issues one tool call and returns the gate's verdict (allow/deny/escalate) plus any approvalId.
func gatedCall(ctx context.Context, sess *mcp.ClientSession, tool string, args map[string]any) (verdict, approvalID string) {
	res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		return "DENY (refused before the tool: " + firstLine(err.Error()) + ")", ""
	}
	meta, _ := res.Meta["stag"].(map[string]any)
	v, _ := meta["verdict"].(string)
	if id, ok := meta["approvalId"].(string); ok {
		approvalID = id
	}
	switch {
	case !res.IsError:
		return "ALLOW → forwarded to the tool", ""
	case v == "escalate":
		return "ESCALATE → held for a human (approvalId " + approvalID + ")", approvalID
	case v == "deny":
		return "DENY → not forwarded", ""
	default:
		return "NOT FORWARDED (" + v + ")", approvalID
	}
}

// humanApprove is the human-in-the-loop step: POST the approve endpoint (mints the ed25519 release) and
// return the signed token the orchestrator presents on retry.
func humanApprove(base, id string) (string, error) {
	resp, err := http.Post(base+"/api/approvals/"+id+"/approve", "application/json", strings.NewReader("{}"))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(b, &out); err != nil || out.Token == "" {
		return "", fmt.Errorf("no token in approve response: %s", strings.TrimSpace(string(b)))
	}
	return out.Token, nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func fatal(f string, a ...any) {
	fmt.Fprintf(os.Stderr, "stag-probe: "+f+"\n", a...)
	os.Exit(1)
}
