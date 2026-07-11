name: ProxyGate
role: component
intent: The gating-proxy core (Planning/17, Slice 0) - the transport-agnostic decision at the tool boundary, with NO MCP dependency. An external agent proposes a tool call {tool, args}; the Gate routes it to a recipe, binds the gated argument as the (untrusted) proposal, runs the deterministic kernel (stag.Eval), and returns a Decision that says whether the call may be FORWARDED to the real downstream tool. THE LOAD-BEARING PROPERTY: a call is forwarded IFF the tool has a recipe AND the kernel verdict is Allow - an unrouted tool (no policy) or any non-Allow verdict (Deny / Escalate / Fault) is NEVER forwarded (complete mediation at the tool boundary, inv 10; fail closed, inv 8). The release events from the crossing are recorded to an egress sink (best-effort, off the enforcement decision, inv 9). No model runs in this path: the agent brings the (untrusted) proposal, the Gate is the deterministic control.
api:
  - "type ToolCall struct { Tool string; Args map[string]string }"
  - "type Route struct { Recipe stag.Recipe; RecipeHash string; GateArg string }"
  - "type Router map[string]Route"
  - "type Sink interface { Record(ctx context.Context, ev stag.ReleaseEvent) error }"
  - "type Decision struct { Tool string; Verdict stag.Verdict; Forward bool; Value string; Events []stag.ReleaseEvent; Fault string }"
  - "type Gate struct { Routes Router; Sink Sink }"
  - func (g Gate) Decide(ctx context.Context, call ToolCall) Decision
concept: deterministic gate at the tool boundary; forward iff routed and Allowed; unknown tool denies; the agent's tool call is the untrusted proposal; no model in the path.
behavior:
  - "DECIDE - ROUTED + ALLOWED FORWARDS: for a ToolCall whose Tool is in Routes, Decide binds Value = call.Args[route.GateArg] (empty string if the arg is absent), runs stag.Eval(route.Recipe, Value, route.RecipeHash), and returns Decision{Tool, Verdict = the kernel verdict, Forward = (Verdict == Allow), Value, Events = the kernel's release events, Fault = the kernel fault}. Forward is true exactly when the verdict is Allow."
  - "DECIDE - UNKNOWN TOOL FAILS CLOSED: for a ToolCall whose Tool is NOT in Routes, Decide returns Decision{Tool, Verdict = Deny, Forward = false, Fault a non-empty 'no recipe' reason}; it does NOT run any recipe and does NOT forward. A tool with no policy is denied, never passed through."
  - "NEVER FORWARDS A NON-ALLOW: for any input, Decision.Forward == true IMPLIES the tool was routed AND stag.Eval on the same recipe+value+hash yields Allow. A Deny, an Escalate, or a Fault yields Forward = false. This is the load-bearing property - the downstream tool is reached only for a cleared call."
  - "RECORDS RELEASE EVENTS: if Sink is non-nil, Decide records each of the kernel's release events to the Sink (the crossing is logged). A Sink error does not change the Decision (egress is best-effort and off the enforcement decision); recording happens for the forwarded/allowed crossings the kernel emitted."
  - "DETERMINISTIC / PURE: Decide with the same Gate and ToolCall returns an equal Decision (the kernel is deterministic); Decide performs no I/O beyond the optional Sink.Record and never panics on any input (missing args, empty tool, nil maps)."
constraints: package proxy at workspaces/stag/proxy (public; import path github.com/scanset/StAG/proxy). Depends on the stag root (Eval, Recipe, Verdict, ReleaseEvent) and stdlib (context). NO MCP dependency, no broker dependency (the Sink interface is local and satisfied structurally by egress.JSONLSink / broker.MemSink). The MCP server/client wiring is a separate quarantined package (proxy/mcp).
