name: SinkGateTest
role: test
intent: Verify the sink-gate decision table and, adversarially, the fail-safe property that no non-authoritative value reaches an authoritative sink at Allow without a release. Small flat table assertions plus a fuzz target that drives arbitrary (subject, sink, released) triples - including out-of-set classes and sensitivities - and asserts the fail-safe invariant holds for every input, since a single Allow of a non-authoritative value at an unreleased authoritative sink would be a product-defining fail-open bug.
api:
  - func TestSinkGate(t *testing.T)
  - func FuzzSinkGate(f *testing.F)
behavior:
  - "BENIGN TABLE (flat statements): GateSink(Untrusted, SinkBenign, false)==Allow; GateSink(Authoritative, SinkBenign, false)==Allow; GateSink(Untrusted, SinkBenign, true)==Allow."
  - "AUTHORITATIVE, NOT RELEASED: GateSink(Authoritative, SinkAuthoritative, false)==Allow; GateSink(Caller, SinkAuthoritative, false)==Deny; GateSink(Untrusted, SinkAuthoritative, false)==Deny (Caller ratified to deny-unless-released 2026-07-01: only an authoritative subject is Allowed here)."
  - "AUTHORITATIVE, RELEASED: GateSink(Untrusted, SinkAuthoritative, true)==Allow; GateSink(Caller, SinkAuthoritative, true)==Allow; GateSink(Authoritative, SinkAuthoritative, true)==Allow."
  - "SENSITIVITY ROUND-TRIP: ParseSinkSensitivity(SinkBenign.String())==SinkBenign and ParseSinkSensitivity(SinkAuthoritative.String())==SinkAuthoritative, nil error each. ERROR CASE: ParseSinkSensitivity(\"unknown\") and ParseSinkSensitivity(\"bogus\") each return a non-nil error; assert err != nil only, do not compare the returned value."
  - "FAIL CLOSED: GateSink(TrustClass(99), SinkAuthoritative, false)==Deny (unknown label). GateSink(Untrusted, SinkSensitivity(99), false)==Deny (unregistered sink)."
  - "FUZZ FuzzSinkGate: seed corners; if len(data)<3 return; derive subject := TrustClass(data[0]%4) (0..3, out-of-set exercised), sink := SinkSensitivity(data[1]%3) (0..2, out-of-set exercised), released := data[2]&1==1. Assert v is a defined verdict, then the FULL contract per input: benign -> Allow; authoritative+released -> Allow; authoritative+!released -> Allow iff subject==Authoritative, else Deny (Caller, Untrusted, and any severed/out-of-set label all Deny); unregistered sink -> Deny. Plus the independent fail-safe negative: v==Allow at an unreleased authoritative sink implies subject==Authoritative. A counterexample is a product-defining fail-open bug."
constraints: package main; standard library only.
