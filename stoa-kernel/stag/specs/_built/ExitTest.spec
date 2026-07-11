name: ExitTest
role: test
intent: Verify the exit terminal — it halts the kernel walk (adds no verdict, records no crossing, later steps do not run), parses as a pure terminal, and is rejected with any illegal key. Also confirms exit is no longer recognized-but-rejected.
api:
  - func TestExitTerminates(t *testing.T)
  - func TestExitOnlyIsVacuousAllow(t *testing.T)
  - func TestExitParsesAsTerminal(t *testing.T)
behavior:
  - "HALT BEFORE A DENY: a hand-built Recipe propose -> authoritative sink (clears) -> exit -> a SECOND authoritative sink that would deny. Eval yields Allow with exactly one crossing and one sink outcome — the exit halts before the denying sink."
  - "VACUOUS ALLOW: a Recipe of a single exit step Evals to Allow, 0 events, no fault (AndAll of no verdicts)."
  - "PARSES AS TERMINAL: a YAML recipe ending in `kind: exit` parses; the last compiled step is NodeExit. An exit carrying an illegal key (e.g. in:) is rejected (not legal). An unknown kind is still rejected as unknown; no ErrNotImplemented sentinel remains."
constraints: package stag (kernel tests over Eval/NodeExit) + package recipe (parser test). stdlib testing only. The parser assertions live alongside the existing recipe tests.
