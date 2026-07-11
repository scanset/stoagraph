name: ProxyGateTest
role: test
intent: Verify the gating-proxy core: a routed+allowed tool call forwards, an unknown tool fails closed (deny, no forward), a denied/escalated call does not forward, release events are recorded, and Decide is deterministic. A fuzz drives arbitrary tool names and argument values against a fixed router and asserts the load-bearing invariant - Forward is true only when the tool is routed AND the kernel independently returns Allow.
api:
  - func TestForwardsRoutedAllowed(t *testing.T)
  - func TestUnknownToolFailsClosed(t *testing.T)
  - func TestDeniedDoesNotForward(t *testing.T)
  - func TestRecordsEvents(t *testing.T)
  - func FuzzForwardIffCleared(f *testing.F)
prelude: "A helper parses a small policy recipe via recipe.Parse: tool write_note gates its text argument against a set_membership rule allowing {hello, status-ok, deploy-done} at an authoritative sink. A spy Sink records the release events it receives. The Router maps write_note -> {that recipe, its SemanticHash, GateArg: text}."
behavior:
  - "FORWARDS ROUTED + ALLOWED: Gate.Decide(ctx, {Tool: write_note, Args: {text: hello}}) returns Verdict Allow, Forward true, Value hello, and a non-empty Events (the authoritative crossing). The gated value is the text argument."
  - "UNKNOWN TOOL FAILS CLOSED: Gate.Decide(ctx, {Tool: delete_everything, Args: {}}) returns Verdict Deny, Forward false, and a non-empty Fault; no recipe runs and nothing forwards - a tool with no policy is denied."
  - "DENIED DOES NOT FORWARD: Gate.Decide(ctx, {Tool: write_note, Args: {text: rm -rf /}}) returns Verdict Deny, Forward false, no events (the value is outside the allowed set, the authoritative sink refuses the crossing)."
  - "RECORDS EVENTS: with a spy Sink, an allowed call records exactly the kernel's release events to the Sink; a denied call records none. The Decision is unchanged whether or not the Sink returns an error."
  - "FUZZ FuzzForwardIffCleared(tool string, arg string): build the fixed router (write_note -> the policy recipe). For a fuzzed tool name and text argument, call Decide. ASSERT: (1) if Decision.Forward is true then the tool is in the router AND stag.Eval(recipe, arg, hash).Verdict == Allow (recomputed independently) - nothing forwards that the kernel did not Allow; (2) an unrouted tool has Forward false and Verdict Deny; (3) Decide never panics; (4) a second identical call returns an equal Decision (determinism). Seed with the allowed value, a denied value, an unknown tool, and empty strings."
constraints: package proxy_test (external test); depends on the proxy package, the stag root (Eval, Verdict), the recipe package (Parse), and stdlib (context, reflect, testing). No MCP dependency.
