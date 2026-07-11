# 09 — First use case and the recipe graph semantics

Recorded 2026-07-01, ratified by Curtis the same day. The first use case is the worked scenario in
`/home/local/Ratchet/docs/preview/ReferenceImplementation.md` (the Zero Trust reference
implementation: an infrastructure agent behind a deterministic assurance layer), reduced to one
StAG recipe. This doc fixes the recipe graph semantics that the use case forced: path-following
evaluation, branch nodes, gate checkpoints, and the structural source of Escalate.

## The mapping (ZT reference to StAG)

| Reference implementation | StAG |
| --- | --- |
| PDP (deterministic policy decision) | GateSink + release rules; gate nodes at checkpoints |
| Policy file (route classes, caps, budgets) | the `rules:` registry (pure data, closed kinds) |
| PIP signed facts vs RAG/telemetry | `ingredients:` trust split (authoritative vs untrusted) |
| Constrained proposal from an untrusted agent | `propose` step (stamped Untrusted, always) |
| Action-chain record | EvalResult.Sinks + Events, document-ordered |
| Coordinator's choice of what to invoke | `branch` node |
| Three test paths: auto / recommend / deny | the Verdict set: Allow / Escalate / Deny |

The three-path column is exact: the reference's auto-executed, recommend-only, and denied outcomes
are the kernel's three verdicts. Until now nothing in the kernel emitted Escalate (GateSink returns
Allow or Deny after the Caller ratification); the gate node below is its ratified structural source.

## Ratified execution semantics (path-following)

1. Eval walks a path through the graph, starting at the first step. Non-taken paths do not
   execute. A step that is not reached produces no outcome, no verdict, no event.
2. Edges are explicit and forward-only. Every non-branch step has at most one outgoing edge: an
   optional `goto: <step-id>` targeting a LATER step, or fall-through to the next step when absent.
   A branch has one edge per case plus a required `default`. Forward-only targets make termination
   structural and keep every lint proof one-pass.
3. The walk ends after the last step executes, or when a gate halts it.
4. `branch` is data-driven routing, never enforcement. It selects on a slot's value using the same
   closed predicate vocabulary as release rules (rule ids from the registry; no expressions, no new
   predicate language). Cases are tested in document order, first match wins, `default` is
   required. A branch may route on untrusted data; that is safe by construction because every path
   still ends at gated sinks, and enforcement never lives in the branch.
5. `gate` is an enforcement checkpoint placed before an action or milestone. It evaluates a
   registry rule against a slot. Pass: the walk continues. Fail: the walk halts on that path with
   verdict Deny, or Escalate when the author explicitly declares it; the default fail outcome is
   Deny (fail closed). Gates are never avoided and never refused: the only way to reach the steps
   beyond a gate is through it, and the linter proves no edge enters the segment between a gate and
   the actions it guards (precise mechanism pinned at spec time).
6. Gate nodes are checkpoints IN ADDITION to the crossing gate, never a replacement. Every
   authoritative sink still runs GateSink with its release rule and emits the ReleaseEvent on an
   allowed crossing. The load-bearing invariant is per-sink and does not depend on graph shape.
7. Escalate emission (the recommend-only path of the reference): a gate with the declared
   escalate-on-fail outcome halts the path and contributes Escalate to the rollup. The U2 algebra
   does the rest: And = max over Allow < Escalate < Deny, so an escalated checkpoint dominates
   clean paths and is dominated by any Deny. The broker routes an Escalate rollup to a human with
   the recorded plan. The exact keyword shape (`on_fail:`) is pinned at spec time.
8. Rollup: EvalResult.Verdict = AndAll over the verdicts of executed gates and sinks only. A
   recipe whose walk ends entirely at benign sinks is Allow with the actuators untouched.
9. ReleaseEvent.Ordering stays the step's document index (int64), which is stable whether or not
   earlier steps were skipped by branching.

## The worked recipe

One recipe, one proposer, all three reference paths. Format is the ratified lint-first shape
(Planning/08).

```yaml
recipe: cdn_remediation
version: 1

# The anomaly reaches the proposer through the broker's context binding (as
# data, never instruction); v1 recipes declare no propose inputs (decision 3),
# so this recipe needs no ingredients section.

rules:
  routes.all:                      # routing breadth: what the branch recognizes
    kind: set_membership
    set: ["class:regional_fallback", "class:edge_only", "class:transcontinental"]
  routes.auto_approvable:          # enforcement: what the gate and the sink clear
    kind: set_membership
    set: ["class:regional_fallback", "class:edge_only"]
  cache.approved_classes:
    kind: set_membership
    set: ["class:release_prewarm"]

steps:
  - id: propose_plan
    kind: propose
    out: plan

  - id: choose_path                # routing, not enforcement
    kind: branch
    in: plan                       # not `on:` - a plain `on` key is a YAML 1.1 bool
    cases:
      - rule: routes.all           # broader than the gate, so out-of-class reroutes
        goto: check_route          # still route TO the checkpoint, which escalates
      - rule: cache.approved_classes
        goto: apply_prefetch
    default: log_only

  - id: check_route                # checkpoint before the milestone
    kind: gate
    in: plan
    rule: routes.auto_approvable
    on_fail: escalate              # the recommend-only path; default is deny

  - id: apply_route
    kind: sink
    in: plan
    field: aws_route_apply.args.route
    sensitivity: authoritative
    rule: routes.auto_approvable   # the crossing gate; Allow emits the ReleaseEvent
    actor: "policy:network_remediation"
    goto: log_only

  - id: apply_prefetch
    kind: sink
    in: plan
    field: edge_cache_prefetch.args.plan
    sensitivity: authoritative
    rule: cache.approved_classes
    actor: "policy:cache_budget"

  - id: log_only
    kind: sink
    in: plan
    field: log.plan
    sensitivity: benign            # recorded, never release-gated
```

(Two corrections made when unit 3's fixture and unit 4's parse rules met this example. First, the
branch case rule is `routes.all`, deliberately broader than the gate's rule: with the original
shared rule the escalate path was unreachable, since any value matching the branch case also
passed the gate. Second, the branch/gate input keyword is `in:`, not `on:` - a plain `on` key is a
YAML 1.1 boolean, which the format's own threat rules reject; the parser refuses `on:` by name
with "use in:". The unused anomaly ingredient was removed: dead declarations are sign-time errors
per decision 7, and v1 propose steps take no inputs.)

The three reference paths land as:

1. Approved prefetch: branch routes to apply_prefetch, the rule releases, GateSink allows, the
   ReleaseEvent records the crossing, fall-through logs. Rollup Allow (auto-executed).
2. Out-of-class reroute: branch routes to check_route, the gate fails with the declared escalate,
   the walk halts before the actuator. Rollup Escalate (recommend-only, handed to a human).
3. Spoofed or unmatched proposal: branch default routes to log_only; no authoritative sink is ever
   reached. The actuator is untouched and the attempt is recorded. (A proposal that somehow reached
   an authoritative sink unreleased would still Deny; the crossing gate does not depend on the
   branch having filtered it.)

## v1 limits (honest notes)

- Eval takes a single proposal string; every propose step writes the same value. Multi-slot
  proposals (a plan with separate route and blast-radius fields) are a broker-phase concern, not a
  kernel one; the use case is written within the one-value limit.
- foreach and exit remain recognized-but-rejected (reject-before-hash). The goto/fall-through law
  above makes exit unnecessary for this use case; it enters scope when a recipe genuinely needs to
  end mid-document.
- Multi-recipe composition (the reference's coordinator invoking sub-flows with linked records) is
  deferred; the branch node covers the single-recipe choice structure.
- The linter's gate-protection proof (law 5) needs a precise mechanism at spec time; the fallback
  guarantee is unaffected because every authoritative sink carries its own crossing gate.

## What this forces on the kernel (build order)

Each is one ladder run (spec_check, red, green, fuzz, quality), security-semantic, so each gets an
adversarial pass:

1. Enum register (Planning/08 decision 1): release RuleKind String() to snake_case plus a
   fail-closed ParseRuleKind; re-run the U4 ladder tail.
2. ReleaseEvent recipe_hash (decision 2): ninth field carrying the semantic hash of the authoring
   document; reopens U6 (hash pin, mutation table, fuzz extend).
3. Recipe graph evolution (U7 v2): Step gains Id, Goto, and the branch/gate fields; NodeBranch and
   NodeGate constants plus NodeKind String/Parse; Eval becomes the path walk defined above;
   EvalResult records gate outcomes; FuzzRecipeEval regenerates random GRAPHS (runtime-chosen
   edges) and re-proves the invariant in both directions plus the rollup law.
4. RecipeParse package: parser and author-time linter per the 18 parser rules and 9 format laws of
   Planning/08 plus the graph laws here.
5. The cdn_remediation recipe becomes the first fixture: parsed, linted, evaluated down all three
   paths in tests.
