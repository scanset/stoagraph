# 19 — Recipe-model extensions: foreach (fan-out) and composition (sub-recipes)

Recorded 2026-07-03. Two recipe-model features raised by Curtis, both kernel-adjacent, both deferred but
worth fixing on paper: **foreach** (bounded iteration — the construct that lets the AI be a real intelligence
source) and **recipe composition** (a sub-recipe as an alternative action path). Neither is implemented; this
records what they are, their status, and how to build them without breaking the kernel's invariants.

## foreach — bounded fan-out; "leverage AI as an intelligence source"

**Status: RESERVED, not implemented.** The kernel has four node kinds (propose / sink / branch / gate) and
`Eval` takes a single proposal string. But the parser already **recognizes-and-rejects** `foreach` (and
`exit`): `kind: foreach` → `ErrNotImplemented` ("recognized vocabulary, rejected before hashing"), with
teaching rejections for `loop` / `with_items` that point at it (`recipe/recipe.go`). So it is earmarked
vocabulary — a recipe cannot use it, but the design reserved it (Planning/01/04/08/09). Ratchet had it.

**Why it matters.** Without it, the model proposes ONE value and the gate runs once — "pick one label." With
foreach, the model proposes a **runtime-sized list** (a plan, N candidate actions, a set of items), and the
gate runs the sub-graph over **each element**. That is what turns the AI from a constrained picker into a
**generative intelligence source**: it can surface many candidates / a multi-step plan, and stag gates
every item deterministically. The list is untrusted; each element is gated independently.

**Semantics (from Ratchet + Planning/01/04).**
- **Bounded.** foreach iterates a runtime list up to a hard cap (like the existing depth/node caps); fail
  closed on exceeding it. No unbounded loops — determinism and termination preserved.
- **Declassifying boundary.** The foreach/child boundary hands off the label: each element enters the child
  sub-graph as untrusted-until-gated (Ratchet drops the label at the call site). The child's sinks/gates
  apply per element.
- **Aggregation.** Per-element verdicts roll up (AndAll — one denied element does not silently pass); events
  collect per element; the record captures each crossing.

**Implementation sketch (a deliberate kernel evolution, like U7v2).** Add a `foreach` node kind; allow a slot
to hold a **collection** (propose can emit a list, or a rule splits a value); the node Evals the child
sub-graph once per element with the element as the crossing value; caps bound the iteration; re-fuzz the
load-bearing invariant over runtime-chosen element counts. Invariant-preserving (each element still crosses
an authoritative sink only via a gate verdict + a recorded event) and termination-bounded.

## recipe composition — a sub-recipe as an alternative action path

**Status: BUILT (U30, 2026-07-04) — compile-time inlining, as recommended below.** A branch case/default uses
`goto_recipe: <name>` / `default_recipe: <name>`; `recipe.Compose(src, resolve)` resolves the child, namespaces
it (per-site prefix over ids/slots/rules + edges), splices it after the parent, and RE-LINTS + RE-HASHES the
composed whole (the inliner is untrusted; the existing linter is the safety net; the parent hash binds the full
expansion). Zero kernel change to `Eval`. It required a real terminal — `exit` (`NodeExit`, also built in U30)
— because the grammar's only terminator was fall-off-the-end, so a spliced child would be reached by
fall-through; `exit` seals a recipe (last-step-exit ⇒ every path halts) and composition requires parent + every
sub-recipe sealed. v1 contract (all linter-enforced, fail-closed): depth-1 (no nested composition), no
self-reference, child is a tail, composed recipes write DISJOINT sink fields (so a child inlines once), and
sub-recipes must be stored before the parent. Wired through recipestore / serve / router (composes at gate
time). See `transcripts/recipe-u30-composition.md`. The design chosen was exactly the recommendation below.

**Original design (kept for the record):** A recipe references another
recipe as a branch target — "if not the primary path, run the escalation_policy sub-recipe."

**Recommended: compile-time inlining.** At save time, a referenced sub-recipe is **expanded** into the parent
(slots namespaced), reached by a `goto_recipe` branch case. The kernel Evals one flattened graph — **zero
kernel change**, every invariant preserved (the linter validates the composed graph: declare-before-use,
gate-protection, the load-bearing sink rule), **termination guaranteed** (acyclic expansion; cycles rejected
at compile time). The parent's recipe hash binds the **composed** policy, so the signed record proves exactly
what ran. The SQLite `recipe_dep(parent, child)` table (Planning/18) tracks the dependency graph — to
recompile parents when a sub-recipe changes and to render composition in the UI.

**Alternative: a runtime `call` node.** Recursively Eval a named recipe. More powerful (dynamic reuse, a
library of composable policies, no re-expansion) but a kernel evolution that needs a **depth bound + cycle
detection** (fail closed on exceed / cycle / missing sub-recipe) and careful verdict-rollup composition. Reach
for it only if dynamic composition is genuinely required.

## The two together

foreach (iterate a list) and composition (reuse a sub-graph) compose: a foreach can run a **sub-recipe per
element** (compose + fan-out) — e.g., the model proposes a remediation plan of N steps, foreach runs the
per-step policy (a sub-recipe) on each. That is the full "propose a plan, gate every action in it" shape.

## foreach v1 — the concrete build design (kernel first, then parser)

The exact semantics to build, chosen to be minimal and invariant-preserving. The list lives as a **JSON-array
string in a slot** — no new slot type; the source is a tool call's list argument (gating proxy) or the model's
list output (model-decision proxy). The kernel does not care where the array came from.

**New node kind `NodeForeach`.** A `foreach` step uses `In` (the slot holding the JSON array) and a new `As`
field (the per-element out-slot). Its **body** is the forward subgraph after it (its `Goto`/fall-through
onward); v1 constraint: foreach is a **tail** construct (its body runs to the recipe's terminals) and the body
contains no nested foreach.

**Walk.** At a `foreach` node the kernel:
1. reads `slot[In]`; a missing/severed slot, a value that is not a JSON array of strings, or a length over the
   kernel cap (a fixed constant, e.g. 64 — author-unraisable, inv 13) → **Fault** (fail closed, inv 8);
2. for each element `e[k]` in order: binds `slot[As] = {Value: e[k], Class: Untrusted, Origin: "foreach"}`
   (the declassifying boundary — each element enters the body fresh and untrusted) and runs an **inner walk**
   of the body, collecting its sinks/gates/events (each event's `Ordering` carries the element index);
3. aggregates: overall `Verdict = AndAll(all per-element verdicts)` — **one denied element denies the batch**;
   `Sinks`/`Gates`/`Events` are the concatenation across elements. An **empty list → Allow, no crossings**
   (vacuous; nothing gated, nothing recorded).

**The load-bearing invariant, per element.** Each element that crosses an authoritative sink at Allow still
requires a gate verdict AND a recorded ReleaseEvent bound to the recipe hash, with the element's *untrusted*
class — the U7v2 invariant, now proven per iteration and `AndAll`-aggregated. Re-fuzzed over runtime-chosen
element lists.

**Build order.** (1) **Kernel** — refactor Eval's walk into a reusable inner-walk function and add the
`NodeForeach` case + list handling + the cap; test with hand-built `stag.Recipe`s (bypassing the parser);
fuzz the per-element invariant. (2) **Parser** — the parser currently *recognizes-and-rejects* `foreach`; make
it ACCEPT `kind: foreach` (with `in`/`as`), build the node, and lint it (body well-formed, no nested foreach,
`As` slot declared-before-use). This mirrors U7v2 (kernel eval) → U8 (parser). Composition (inlining) is a
separate, later parser unit.

## Status

Both deferred; sequence after the Adapters slice (Planning/18) unless prioritized. foreach is the higher-value
of the two (it unlocks list/plan proposals); composition is the DRY/reuse convenience. Both build on the
`/recipes` authoring page and the SQLite store.
