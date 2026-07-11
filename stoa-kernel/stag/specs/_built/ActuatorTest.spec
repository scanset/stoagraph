name: ActuatorTest
role: test
intent: Verify complete mediation at the actuator boundary: FireCleared fires an actuator ONLY for a cleared action and NEVER for a denied or recommended one - even when they share a Ref with a bound actuator. The fuzz drives random decisions and asserts every fire corresponds to a cleared action, so no Deny or Escalate can ever cause a real effect. Also verify the stub touches nothing, unbound cleared sinks are a noted no-op, and an actuator error is surfaced not swallowed.
api:
  - func TestStub(t *testing.T)
  - func TestFireOnlyCleared(t *testing.T)
  - func TestActuatorError(t *testing.T)
  - func FuzzFireCleared(f *testing.F)
prelude: "A spy actuator records each Fire (the ref it is bound to + the value) into a shared slice, so a test can assert exactly which actuators fired. A decision helper builds a broker.Decision with given Cleared/Denied/Recommend refs and a proposal value."
behavior:
  - "STUB: Stub{Name:\"remediate\"}.Fire(ctx, \"restart_service\") returns a non-empty line containing \"remediate\" and \"restart_service\" and a nil error; it performs no I/O."
  - "FIRE ONLY CLEARED: a decision with Cleared refs [\"a\"], Denied refs [\"b\"], Recommend refs [\"c\"], proposal value \"restart_service\", and a Registry binding a spy to EACH of a,b,c. FireCleared returns exactly one Result (for a) with Fired:true and Value \"restart_service\"; the spy log contains ONLY a (b and c NEVER fired) - a denied or recommended action does not fire even though its actuator is bound."
  - "UNBOUND CLEARED: a decision with Cleared refs [\"x\"] and an EMPTY registry: FireCleared returns one Result{Ref:\"x\", Fired:false}; nothing fires; no error, no panic."
  - "ACTUATOR ERROR SURFACED: a cleared decision whose bound actuator returns an error: the Result has Fired:true and a non-empty Err; a second cleared action after it is still processed (its Result present) - one failing effect does not drop the others."
  - "FUZZ FuzzFireCleared - complete mediation. From fuzzed bytes build lists of cleared, denied, and recommend refs and a proposal value; bind a spy to EVERY ref that appears in any list; build the Decision; run FireCleared. ASSERT: (1) every ref recorded by a spy appears in the Cleared list (nothing outside Cleared ever fired); (2) the number of fires equals the number of cleared actions whose ref is bound; (3) each Result's Value equals the proposal value; (4) FireCleared never panics; (5) a second run yields equal Results (determinism). Seed with disjoint lists, a ref shared between Cleared and Denied, and empty lists."
constraints: package actuator_test (external test); depends on the actuator package, the broker package, the model package (Proposal), the stag root (Verdict/SinkSensitivity for building Actions), context, reflect, strings, testing.
