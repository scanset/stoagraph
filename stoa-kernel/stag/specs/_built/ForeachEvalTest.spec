name: ForeachEvalTest
role: test
intent: Verify the kernel foreach node: it gates each element of a JSON-array proposal, AndAll-aggregates (one denied element denies the batch), emits one release event per released element, handles the empty list, and fails closed on a non-array value / over-cap / nesting / severed slot. A fuzz drives arbitrary element lists and asserts the aggregate verdict and the release-event count match a per-element re-computation. A non-foreach recipe still evaluates unchanged.
api:
  - func TestForeachAllAllowed(t *testing.T)
  - func TestForeachOneDeniedDeniesBatch(t *testing.T)
  - func TestForeachEmptyList(t *testing.T)
  - func TestForeachFailsClosed(t *testing.T)
  - func TestNodeKindForeachParse(t *testing.T)
  - func FuzzForeach(f *testing.F)
prelude: "A hand-built foreach recipe: propose out=plan; foreach in=plan as=item; sink in=item field=exec.action sensitivity=authoritative rule=action.allowed actor policy:x, where action.allowed is a set_membership rule over {restart, scale, clear}. Eval is called with a JSON-array string proposal. A semantic hash string is passed through as recipeHash."
behavior:
  - "ALL ALLOWED: Eval(recipe, `[\"restart\",\"scale\"]`, hash) -> Verdict Allow; exactly 2 release events (one per element), each with SubjectClass Untrusted and RecipeHash == hash and distinct Ordering; 2 authoritative sink outcomes both Allow."
  - "ONE DENIED DENIES THE BATCH: Eval(recipe, `[\"restart\",\"bad_cmd\"]`, hash) -> Verdict Deny (AndAll); exactly 1 release event (for restart only - bad_cmd is outside the set and emits none); the batch verdict is Deny even though restart alone would allow."
  - "EMPTY LIST: Eval(recipe, `[]`, hash) -> Verdict Allow, 0 events, 0 sinks/gates. Nothing gated."
  - "FAILS CLOSED: Eval(recipe, `not-json`, hash) and Eval(recipe, `[1,2]` (numbers, not strings), hash) and Eval(recipe, a JSON array of more than foreachCap strings, hash) each yield a non-empty Fault and Verdict Deny with 0 release events. A recipe with a foreach node whose body contains another foreach also Faults (no nesting). None of these panic."
  - "NODE KIND: ParseNodeKind(\"foreach\") returns NodeForeach with a nil error; NodeForeach.String() == \"foreach\"; an unknown kind still errors."
  - "NON-FOREACH UNCHANGED: a simple propose->authoritative-sink recipe (no foreach) evaluates to the same result as before the change - Eval(thatRecipe, \"restart\", hash) with the same rule yields Allow + 1 event (regression guard that the walk refactor preserved behaviour)."
  - "FUZZ FuzzForeach(data []byte): build 0..N element strings from data (some in {restart,scale,clear}, some not), marshal them to a JSON array, Eval the foreach recipe. ASSERT: (1) never panics; (2) if the element count exceeds foreachCap -> Fault/Deny; else (3) the number of release events equals the number of elements in the allowed set, and (4) the overall Verdict is Allow iff EVERY element is in the allowed set (recompute independently), else Deny; (5) a re-Eval is identical (determinism). Seed with all-allowed, one-denied, empty, and an over-cap list."
constraints: package stag_test or the kernel's internal test package (matching the existing Eval tests); depends on the stag root (Eval, Recipe, Step, NodeForeach, NodePropose, NodeSink, ReleaseRule, Verdict, TrustClass) and stdlib (encoding/json, testing). Hand-built recipes only (no parser).
