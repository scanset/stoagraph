name: ForeachParseTest
role: test
intent: Verify the parser accepts foreach and lints it: a foreach recipe parses and Evals per element; the linter rejects a missing/duplicate as, an undeclared in, an illegal key, and more than one foreach; exit stays rejected; the existing fixtures are unchanged. Confirms foreach is authorable end-to-end.
api:
  - func TestForeachRecipeParsesAndEvals(t *testing.T)
  - func TestForeachLintRejects(t *testing.T)
  - func TestExitStillRejected(t *testing.T)
prelude: "A valid foreach recipe YAML: propose out=plan; a foreach step {kind: foreach, in: plan, as: item}; an authoritative sink in=item gated by a set_membership rule over {restart, scale}. Poison variants mutate one line for the reject cases."
behavior:
  - "PARSES + EVALS: Parse(the foreach recipe) succeeds; the compiled Recipe's foreach step has Kind NodeForeach with In=plan and As=item. stag.Eval(p.Recipe, `[\"restart\",\"scale\"]`, p.SemanticHash) yields Verdict Allow with 2 release events; Eval(..., `[\"restart\",\"nope\"]`, ...) yields Deny with 1 event. ParseDraft returns no error and (for the clean recipe) no warnings."
  - "LINT REJECTS (each a non-nil error, no compile): (a) a foreach missing `as`; (b) a foreach missing `in`; (c) a foreach whose `as` duplicates an existing slot; (d) a foreach whose `in` is an undeclared slot; (e) a foreach with an illegal key (e.g. rule:); (f) a recipe with TWO foreach steps (no nesting in v1)."
  - "EXIT STILL REJECTED: a recipe with kind: exit returns an error that errors.Is ErrNotImplemented; foreach no longer does."
  - "REGRESSION: the existing fixture recipe (the non-foreach cdn_remediation, or the skeleton) still parses exactly as before (unchanged); a body step reading the foreach `as` slot is accepted (declare-before-use satisfied by the foreach defining it)."
constraints: package recipe (internal test, matching the existing recipe_test.go) or recipe_test; depends on the recipe package (Parse, ParseDraft, ErrNotImplemented) and the stag root (Eval, NodeForeach, Verdict) and stdlib (errors, strings, testing).
