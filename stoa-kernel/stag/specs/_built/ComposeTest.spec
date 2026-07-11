name: ComposeTest
role: test
intent: Verify recipe composition (compile-time inlining) — a parent inlines a sub-recipe via goto_recipe/default_recipe, the composed graph re-lints and Evals as the child would, the parent hash binds the full expansion, and the fail-closed cases (missing/self/nested/cycle/grammar) are refused. Includes a fuzz target over composed graphs.
api:
  - func TestComposeInlinesChild(t *testing.T)
  - func TestComposeDefaultRecipe(t *testing.T)
  - func TestComposeHashBindsExpansion(t *testing.T)
  - func TestComposeFailClosed(t *testing.T)
  - func TestComposeGrammarAndRegression(t *testing.T)
  - func FuzzCompose(f *testing.F)
prelude: "A parent recipe `router` with a branch on a slot; one case routes with `goto_recipe: escalate_policy`. A child recipe `escalate_policy`: propose out=plan; an authoritative sink gated by a set_membership rule. A `resolve` closure backed by a map[string][]byte returns child sources by name. Reject/​poison variants: missing child, self-reference (goto_recipe: router), a child that itself has goto_recipe (nested), goto_recipe on a non-branch step, goto_recipe alongside goto in one case."
behavior:
  - "INLINES + EVALS: Compose(parent, resolve) succeeds; the composed Parsed.Recipe contains the parent's steps PLUS the child's steps under a namespaced prefix (distinct step ids, distinct slots). stag.Eval over a proposal that routes into the inlined case gates exactly as the standalone child would (same verdict + same release-event count for the crossing); a proposal routing elsewhere is unaffected."
  - "DEFAULT_RECIPE: a branch with `default_recipe: escalate_policy` inlines the child on the default path; a proposal matching no case routes into the inlined child and gates as the child would."
  - "HASH BINDS EXPANSION: the parent's SemanticHash over the composed graph is stable across re-Compose (deterministic) and CHANGES when the child source changes (edit the child's rule set → different parent hash); a parent inlining child A hashes differently from the same parent inlining child B. A no-composition recipe's hash is byte-identical to Parse of the same source (regression)."
  - "FAIL CLOSED: Compose returns a non-nil error (no Parsed) for — (a) a goto_recipe whose child name does not resolve; (b) self-reference (goto_recipe naming the parent); (c) a child that itself uses goto_recipe/default_recipe (nested composition); (d) a resolve that yields an unparseable child (the child's parse error surfaces). Each is refused before any recipe is produced."
  - "GRAMMAR + REGRESSION: goto_recipe on a propose/sink/foreach step is rejected; a case with BOTH goto and goto_recipe is rejected; a branch with BOTH default and default_recipe is rejected. recipe.Parse of a composed recipe (goto_recipe present) errors clearly (reject-all resolver). Every existing recipe fixture parses through Compose(reject-all) exactly as before."
  - "FUZZ (FuzzCompose): seed with the parent+child pair and mutations; for arbitrary bytes as parent AND as child source (via a fixed one-entry resolver), Compose never panics and never returns a Parsed together with a non-nil error; any returned Parsed re-Composes to the SAME SemanticHash (determinism) and passes a structural sanity check (all edges forward, all step ids unique)."
constraints: package recipe (internal test alongside recipe_test.go); depends on recipe (Compose, Parse, ErrNotImplemented) + stag root (Eval, Verdict, NodeForeach) + stdlib (errors, strings, testing). The fuzz target must be deterministic (no Date.now/random) and bounded.
