// Package proxy is the gating-proxy core (Planning/17, Slice 0): the
// transport-agnostic decision at the tool boundary, with NO MCP dependency. An
// external agent proposes a tool call; the Gate routes it to a recipe, runs the
// deterministic kernel, and says whether the call may be FORWARDED to the real
// downstream tool. A call is forwarded IFF it is routed AND the kernel verdict is
// Allow — an unrouted tool or any non-Allow verdict is never forwarded.
package proxy

// file-kw: gating proxy tool boundary route recipe eval forward-iff-cleared fail-closed no-model mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	stag "github.com/scanset/stoagraph/stoa-kernel/stag"
)

// MetaApprovalToken is a gate-only argument the ORCHESTRATOR attaches on an approved retry: it
// carries the signed release token. It is bound for gating (a recipe proposes it) but is NEVER
// forwarded downstream — the mcpgate strips it before the real tool call. A tool's real args
// never include it.
const MetaApprovalToken = "approval_token"

// signedPlaceholder is the literal a recipe author writes for a signed_equality rule whose
// expected value is minted per-request by a human approval (Stage 5). The proxy ALWAYS resolves
// it to the looked-up token (or "" when unapproved -> the rule fails closed); the literal never
// reaches the kernel, so it can't be presented to bypass the gate.
const signedPlaceholder = "$approved"

// kw: tool call name args from the untrusted agent
type ToolCall struct {
	Tool string
	Args map[string]string
}

// kw: route recipe hash gated-arg for a tool
type Route struct {
	Recipe     stag.Recipe
	RecipeHash string
	GateArg    string
	RecipeName string // for audit/approval display; not load-bearing
}

// Approvals is the gate-side store for the human-approval loop (Stage 5). It is OPTIONAL: a nil
// Approvals disables the loop (an escalate stays a dead-end, as before). Primitive signatures so
// *store.Store satisfies it without either package importing the other.
type Approvals interface {
	// LookupApproved returns the signed release token for an action fingerprint iff a row for it
	// is currently `approved`. ok=false (token "") otherwise.
	LookupApproved(ctx context.Context, fingerprint string) (token string, id string, ok bool, err error)
	// RecordPending logs an escalated action awaiting approval (idempotent per id). created=true
	// means a NEW approval needs a human -> the caller fires the webhook.
	RecordPending(ctx context.Context, id, tool, fingerprint, argsJSON, recipe, recipeHash string) (created bool, err error)
	// Consume marks an approved release spent (one-time); a replay then re-escalates.
	Consume(ctx context.Context, id string) error
}

// PendingNotice is what the gate hands OnEscalate when a fresh approval is recorded (the webhook
// payload). It is push-notification only; the store row is the source of truth.
type PendingNotice struct {
	ID          string `json:"id"`
	Tool        string `json:"tool"`
	Fingerprint string `json:"fingerprint"`
	ArgsJSON    string `json:"args"`
	Recipe      string `json:"recipe"`
}

// kw: router tool name to route
type Router map[string]Route

// kw: sink egress record release event (egress.JSONLSink / broker.MemSink satisfy this)
type Sink interface {
	Record(ctx context.Context, d stag.DecisionRecord) error
}

// kw: decision tool verdict forward value events fault approval-id
type Decision struct {
	Tool       string
	Verdict    stag.Verdict
	Forward    bool
	Value      string
	Events     []stag.ReleaseEvent
	Fault      string
	ApprovalID string // set when an escalate is (or awaits) a human approval; "" otherwise
}

// kw: gate routes sink deterministic tool-boundary approvals notify
type Gate struct {
	Routes     Router
	Sink       Sink
	Approvals  Approvals                                  // optional: enables the escalate->approval loop (Stage 5)
	OnEscalate func(ctx context.Context, n PendingNotice) // optional: push notify (webhook) on a fresh escalation
}

// kw: decide route eval forward-iff-cleared record fail-closed approval-loop
func (g Gate) Decide(ctx context.Context, call ToolCall) Decision {
	route, ok := g.Routes[call.Tool]
	if !ok {
		// fail closed (inv 8/10): a tool with no policy is denied, never forwarded. It is still RECORDED
		// — "the agent reached for a tool it was never granted" is exactly the evidence an auditor wants,
		// and dropping it would leave the most suspicious call of all invisible.
		d := Decision{Tool: call.Tool, Verdict: stag.Deny, Forward: false, Fault: "no recipe for tool " + call.Tool}
		g.record(ctx, d, "", "")
		return d
	}

	// GateArg is one arg name (single-arg), or a comma-separated list (multi-arg): each listed
	// arg binds a `propose out: <arg>` slot, so one recipe can decide from several arguments
	// (e.g. "namespace,replicas"). A missing arg binds "" and fails its rule.
	names := splitGateArg(route.GateArg)
	args := make(map[string]string, len(names))
	parts := make([]string, 0, len(names))
	for _, a := range names {
		args[a] = call.Args[a]
		if a != MetaApprovalToken { // the token is a gate-only meta arg — never shown in the audit value
			parts = append(parts, a+"="+args[a])
		}
	}
	// Audit/human value: single-arg shows the raw value ("hello"); multi-arg shows "k=v k=v".
	value := args[names[0]]
	if strings.Contains(route.GateArg, ",") {
		value = strings.Join(parts, " ")
	}

	// STAGE 5: if the recipe has a signed_equality "$approved" gate, resolve it against the
	// approval store. LookupApproved returns the human-minted token for this exact action iff it
	// is currently approved; we ALWAYS substitute (token or "") so the placeholder never evals.
	recipe := route.Recipe
	fingerprint, approvedID := "", ""
	needsApproval := g.Approvals != nil && recipeHasApprovalGate(route.Recipe)
	if needsApproval {
		// fingerprint binds the WHOLE action (all call args, minus the token), not just the gated
		// subset — so an approval authorizes exactly this call, not every call sharing a gated value.
		fingerprint = Fingerprint(call.Tool, call.Args)
		token := ""
		if tok, id, okA, err := g.Approvals.LookupApproved(ctx, fingerprint); err == nil && okA {
			token, approvedID = tok, id
		}
		recipe = resolveApproved(route.Recipe, token)
	}

	var res stag.EvalResult
	if strings.Contains(route.GateArg, ",") {
		res = stag.EvalArgs(recipe, args, route.RecipeHash)
	} else {
		res = stag.Eval(recipe, args[names[0]], route.RecipeHash) // single-arg binds the one proposal everywhere
	}

	// forward IFF the whole-recipe verdict is Allow. A Deny, an Escalate, or a Fault never
	// reaches the downstream tool (complete mediation at the boundary).
	forward := res.Verdict == stag.Allow

	if needsApproval {
		switch {
		case res.Verdict == stag.Allow && approvedID != "":
			_ = g.Approvals.Consume(ctx, approvedID) // one-time: burn the release on use
		case res.Verdict == stag.Escalate && approvedID == "":
			id := idFor(fingerprint)
			argsJSON := marshalArgs(call.Args) // the full action, for the dashboard
			if created, _ := g.Approvals.RecordPending(ctx, id, call.Tool, fingerprint, argsJSON, route.RecipeName, route.RecipeHash); created && g.OnEscalate != nil {
				g.OnEscalate(ctx, PendingNotice{ID: id, Tool: call.Tool, Fingerprint: fingerprint, ArgsJSON: argsJSON, Recipe: route.RecipeName})
			}
		}
	}

	d := Decision{Tool: call.Tool, Verdict: res.Verdict, Forward: forward, Value: value, Events: res.Events, Fault: res.Fault, ApprovalID: approvalIDForView(res.Verdict, approvedID, fingerprint, needsApproval)}
	g.record(ctx, d, route.RecipeName, route.RecipeHash)
	return d
}

// record writes exactly ONE leaf per decision — allow, deny, or escalate alike.
//
// Releases ride along ONLY when the call was forwarded. A multi-arg recipe evaluates every sink, so a
// DENIED call can still have individually-cleared sinks (owner=mallory fails while repo=stoagraph
// passes). Recording those as releases would put a crossing in the tamper-evident log that never
// happened — the audit would claim the agent read a repo the gate actually blocked. The record states
// what HAPPENED, not what merely evaluated.
//
// Egress stays best-effort and off the enforcement path (inv 9): a sink error never changes the verdict.
func (g Gate) record(ctx context.Context, d Decision, recipeName, recipeHash string) {
	if g.Sink == nil {
		return
	}
	rec := stag.DecisionRecord{
		Tool:       d.Tool,
		Verdict:    d.Verdict.String(),
		Forwarded:  d.Forward,
		Value:      d.Value,
		Recipe:     recipeName,
		RecipeHash: recipeHash,
		Fault:      d.Fault,
	}
	if d.Forward { // released iff forwarded
		rec.Events = d.Events
	}
	_ = g.Sink.Record(ctx, rec)
}

// splitGateArg parses a GateArg into its arg names: one name (single-arg) or a comma-separated
// list (multi-arg). Empty entries are dropped; a single non-comma arg returns one name.
func splitGateArg(gateArg string) []string {
	out := make([]string, 0, 2)
	for _, a := range strings.Split(gateArg, ",") {
		if a = strings.TrimSpace(a); a != "" {
			out = append(out, a)
		}
	}
	if len(out) == 0 {
		out = append(out, "") // preserves the "missing arg binds \"\"" fail-closed behavior
	}
	return out
}

// Fingerprint canonicalizes an action (tool + its args, EXCLUDING the approval-token meta arg)
// into a stable string. A human approves this exact tuple; the signed release binds to it, and a
// retry with the same args reproduces it. Sorted keys => order-independent.
func Fingerprint(tool string, args map[string]string) string {
	keys := make([]string, 0, len(args))
	for k := range args {
		if k == MetaApprovalToken || k == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(tool)
	for _, k := range keys {
		b.WriteByte('\x1f')
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(args[k])
	}
	return b.String()
}

// idFor is the approval id: a short, stable digest of the fingerprint, so re-escalating the same
// action maps to one approval row (idempotent).
func idFor(fingerprint string) string {
	sum := sha256.Sum256([]byte(fingerprint))
	return hex.EncodeToString(sum[:8])
}

// recipeHasApprovalGate reports whether any rule in the recipe is a signed_equality "$approved"
// placeholder — i.e. this recipe participates in the human-approval loop. Cheap; walks steps once.
func recipeHasApprovalGate(r stag.Recipe) bool {
	for i := range r.Steps {
		if isApprovedRule(r.Steps[i].Rule) {
			return true
		}
		for j := range r.Steps[i].Cases {
			if isApprovedRule(r.Steps[i].Cases[j].Rule) {
				return true
			}
		}
	}
	return false
}

func isApprovedRule(rule *stag.ReleaseRule) bool {
	return rule != nil && rule.Kind == stag.RuleSignedEquality && rule.Signed == signedPlaceholder
}

// resolveApproved returns a shallow clone of the recipe with every signed_equality "$approved"
// rule's expected value set to token (which is "" when the action is not currently approved, so
// the rule fails closed). Only rules that need substituting get fresh pointers — the shared
// parsed recipe (held by the router across calls) is never mutated.
func resolveApproved(r stag.Recipe, token string) stag.Recipe {
	steps := make([]stag.Step, len(r.Steps))
	copy(steps, r.Steps) // Step is a value; its *Rule pointers are shared until we replace them
	for i := range steps {
		if isApprovedRule(steps[i].Rule) {
			nr := *steps[i].Rule
			nr.Signed = token
			steps[i].Rule = &nr
		}
		if len(steps[i].Cases) > 0 {
			cs := make([]stag.Case, len(steps[i].Cases))
			copy(cs, steps[i].Cases)
			for j := range cs {
				if isApprovedRule(cs[j].Rule) {
					nr := *cs[j].Rule
					nr.Signed = token
					cs[j].Rule = &nr
				}
			}
			steps[i].Cases = cs
		}
	}
	return stag.Recipe{Ingredients: r.Ingredients, Steps: steps}
}

// marshalArgs renders the gated args as compact JSON for the approval row (dashboard display);
// the approval-token meta arg is omitted (it's not part of the action being approved).
func marshalArgs(args map[string]string) string {
	m := make(map[string]string, len(args))
	for k, v := range args {
		if k != MetaApprovalToken && k != "" {
			m[k] = v
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// approvalIDForView surfaces the approval id on the Decision so the caller/audit can point a human
// at the pending item. Only meaningful for approval-gated escalations.
func approvalIDForView(v stag.Verdict, approvedID, fingerprint string, needsApproval bool) string {
	if !needsApproval {
		return ""
	}
	if v == stag.Escalate {
		return idFor(fingerprint) // the pending id a human must act on
	}
	return approvedID // the id we released (and consumed), if any
}
