# Writing recipes

A **recipe** is a stag policy: a small, closed graph that a proposed tool call walks through to a
verdict. It is authored in YAML, validated by a linter, and executed by a deterministic kernel — no
model, no I/O. Same recipe + same call → same verdict, always.

## The mental model

A tool call arrives as a set of named **arguments** (e.g. `namespace`, `replicas`). A recipe:

1. **proposes** those arguments as **untrusted** values (they came from the agent — you never trust them),
2. optionally **branches** on them and **gates** them against rules, and
3. **sinks** them: an *authoritative* sink is the moment a value would actually reach the real tool.

The load-bearing invariant: **no untrusted value reaches an authoritative sink without a rule release
and a recorded crossing.** Everything below serves that guarantee.

The verdict of the whole recipe is the AND of its steps: **allow** only if every crossing was
released; otherwise **deny**, **escalate**, or **fault**.

## Anatomy

```yaml
recipe: my_policy       # name (also the id you route tools to)
version: 1
rules:                  # named, reusable predicates
  ns.safe: {kind: set_membership, set: ["dev", "staging"]}
  count.ok: {kind: numeric_range, min: 0, max: 5}
steps:                  # the graph, top to bottom
  - {id: propose_ns,   kind: propose, out: namespace}
  - {id: propose_n,    kind: propose, out: replicas}
  - {id: apply, kind: sink, in: replicas, field: k8s.scale.apply,
     sensitivity: authoritative, rule: count.ok, actor: "policy:platform"}
```

## Rules — the three closed predicates

A rule decides whether a value is **released**. There are exactly three kinds (a closed set keeps the
policy language auditable):

| Kind | Passes when | Example |
| --- | --- | --- |
| `set_membership` | the value is one of a fixed set | `{kind: set_membership, set: ["dev", "staging"]}` |
| `numeric_range` | the value is an integer in `[min, max]` (canonical form only) | `{kind: numeric_range, min: 0, max: 5}` |
| `signed_equality` | the value byte-equals a pinned/approved value | `{kind: signed_equality, signed: "$approved"}` |

To DENY everything, use an unsatisfiable set: `{kind: set_membership, set: ["__never__"]}`.

## Steps — the six node kinds

The graph is built from a closed set of node kinds. Edges are **forward-only** (a `goto` always
points to a later step), so a recipe is a DAG that always terminates.

- **`propose`** — bind a tool argument as an untrusted value.
  `{id: p, kind: propose, out: <slot>}`  (`out` is the argument name)
- **`branch`** — route on a value; first matching case wins, else `default`.
  ```yaml
  {id: route, kind: branch, in: namespace,
   cases: [{rule: ns.safe, goto: apply}, {rule: ns.prod, goto: escalate_gate}],
   default: deny_sink}
  ```
- **`gate`** — guard the steps that follow. If the value fails `rule`, the recipe stops with
  `on_fail`.
  `{id: g, kind: gate, in: namespace, rule: never, on_fail: escalate}`  (`on_fail`: `escalate` | `deny`)
- **`sink`** — the crossing. `sensitivity: authoritative` is a real release to the tool (releases only
  if `rule` passes; records a crossing); `benign` is a non-authoritative read.
  `{id: apply, kind: sink, in: replicas, field: k8s.scale.apply, sensitivity: authoritative, rule: count.ok, actor: "policy:platform", goto: done}`
- **`exit`** — a terminal node. `{id: done, kind: exit}`
- **`foreach`** — iterate a body over elements (advanced; see the composed examples).

## Verdicts

| Verdict | Meaning | Forwarded to the tool? |
| --- | --- | --- |
| `allow` | every crossing released | **yes** |
| `deny` | a crossing was refused | no |
| `escalate` | a gate deferred to a human (approval queue) | no (until approved) |
| `fault` | the recipe or call was malformed | no (fail closed) |

## The linter (why a recipe is safe by construction)

Before a recipe runs, it must pass structural checks — a policy that could leak is a **rejected
recipe**, not a runtime surprise:

- **declare-before-use** — a slot must be `propose`d before it is read.
- **definite-assignment** — every value a sink reads is guaranteed to be bound on every path.
- **forward-only edges** — `goto` points forward; no cycles; the graph terminates.
- **unique fields** — no two authoritative sinks claim the same field.
- **gate-protection** — the authoritative sinks a gate guards are only reachable *through* that gate.

## Worked examples

**Hard deny** — deleting a namespace is never routine:

```yaml
recipe: k8s_delete_ns_policy
version: 1
rules:
  never: {kind: set_membership, set: ["__never__"]}
steps:
  - {id: propose_ns, kind: propose, out: ns}
  - {id: attempt, kind: sink, in: ns, field: k8s.delete_namespace,
     sensitivity: authoritative, rule: never, actor: "policy:platform"}
```

**Tiered by namespace** — dev/staging auto, prod escalates, everything else denied:

```yaml
recipe: k8s_restart_policy
version: 1
rules:
  ns.safe:  {kind: set_membership, set: ["dev", "staging"]}
  ns.prod:  {kind: set_membership, set: ["prod"]}
  ns.never: {kind: set_membership, set: ["__never__"]}
steps:
  - {id: propose_ns, kind: propose, out: ns}
  - id: route
    kind: branch
    in: ns
    cases:
      - {rule: ns.safe, goto: apply}      # dev/staging -> release
      - {rule: ns.prod, goto: escgate}    # prod        -> escalate
    default: block                         # else        -> deny
  - {id: apply,  kind: sink, in: ns, field: k8s.restart.apply,   sensitivity: authoritative, rule: ns.safe, actor: "policy:platform", goto: exit_ok}
  - {id: exit_ok, kind: exit}
  - {id: block,  kind: sink, in: ns, field: k8s.restart.blocked, sensitivity: authoritative, rule: ns.safe, actor: "policy:platform", goto: exit_deny}
  - {id: exit_deny, kind: exit}
  - {id: escgate, kind: gate, in: ns, rule: ns.never, on_fail: escalate}
  - {id: exit_esc, kind: exit}
```

## Multiple arguments, and human approval

- **Multi-argument gating** — route a tool to several arguments at once (e.g. `namespace,replicas`);
  each `propose out: X` binds the argument named `X`, so one recipe decides from the whole action
  (e.g. "scaling *prod* escalates regardless of the count").
- **Human approval** — a `signed_equality` gate whose `signed:` value is `"$approved"` escalates until
  a human approves; approval mints a signed release for that exact action, and the retried call passes.

## Tips

- YAML 1.1 gotcha: a bare slot named `n`, `y`, or `no` parses as a boolean — quote it or rename it
  (e.g. use `replicas`, not `n`).
- Prefer explicit `exit` nodes per branch — it makes the graph (and the linter) unambiguous.
- Validate a draft before routing it: `POST /api/recipes` returns `{valid, error}`; the console shows
  the linter result live.
