name: Exit
role: component
intent: A real terminal node kind, `exit`, that halts a path explicitly (the grammar reserved it; composition requires it). Today a path terminates only by falling off the end of the step list or by a gate-halt; that makes it impossible to seal a recipe so another recipe can be appended after it (composition). NodeExit halts the walk, adds NO verdict and records NO crossing. With forward-only edges, a recipe whose LAST step is exit has every path end in exit-or-gate-halt (sealed) — the precondition for safe inlining.
api:
  - "stag: new NodeExit node kind; String()==\"exit\"; ParseNodeKind(\"exit\")==NodeExit; walk `case NodeExit: break walk` (halt)."
  - "recipe: `kind: exit` parses; legal keys are ONLY {id, kind} (no in/out/goto); it is a pure terminal."
concept: exit is a terminal — it stops the path with no verdict and no crossing; steps physically after it on that path do not run.
behavior:
  - "KERNEL HALT: at a NodeExit the walk stops. It contributes no Verdict (AndAll over an empty tail is vacuous Allow) and appends no SinkOutcome/ReleaseEvent. A sink placed after an exit on the same path never runs (verdict/crossings reflect only what ran before the exit)."
  - "PARSER TERMINAL: a step {id, kind: exit} parses to Step{Kind: NodeExit}; canonical form is {id, kind}; the compiled step carries no fields. Any other key (in/out/goto/rule/...) on an exit is rejected (not legal). exit is no longer recognized-but-rejected."
  - "LINT AS TERMINAL: exit consumes no slot (skipped in declare-before-use), has NO successor in reachability and definite-assignment (it does not fall through to i+1), and ENDS a gate's guarded segment. A recipe may end with exit; an exit mid-list halts that path (later steps reached only via other edges)."
  - "SEALED: with forward-only edges, `last step is exit` ⇒ every path halts at a gate-fail or reaches the exit ⇒ no path falls off the end. This is the property composition depends on (a sealed recipe can have another appended after it with no fall-through across the seam)."
constraints: package stag (kernel) + package recipe (parser/linter). No new dependency. Enables Compose (recipe composition); on its own it is a small terminal primitive. Removes the last recognized-but-rejected kind, so ErrNotImplemented is retired.
