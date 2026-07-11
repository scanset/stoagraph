name: ForeachParse
role: component
intent: Make the recipe parser ACCEPT `kind: foreach` (it currently recognizes-and-rejects it) so foreach is authorable in YAML and usable end-to-end (author in the console -> route -> gate a list). A foreach step declares `in` (the list slot) and `as` (the per-element out-slot); the parser builds a stag.Step{Kind: NodeForeach, In, As} and the linter treats foreach as DEFINING `as` (untrusted) and USING `in` (declare-before-use + definite-assignment on every path), so the body can read `as`. v1 allows AT MOST ONE foreach per recipe (no nesting - the kernel Faults on a nested foreach; the linter rejects it up front). `kind: exit` remains recognized-but-rejected. Everything else about the grammar/linter/hashing is unchanged.
api:
  - "recipe.Parse / recipe.ParseDraft now accept a foreach step (previously ErrNotImplemented)"
  - "foreach step legal keys: id, kind, in, as, goto"
  - "the compiled stag.Recipe carries Step{Kind: NodeForeach, In: <list slot>, As: <element slot>, Goto: optional}"
concept: author foreach in YAML; foreach defines `as` and uses `in`; at most one foreach (v1, no nesting); exit still rejected; the parsed recipe Evals as the kernel foreach (U28).
behavior:
  - "ACCEPTS FOREACH: a recipe with a step {id, kind: foreach, in: <declared slot>, as: <name>} parses without error; the compiled Recipe has a Step with Kind NodeForeach, In and As set, and its body (the following steps) validated normally. Eval of the parsed recipe with a JSON-array proposal gates each element (matching U28)."
  - "LEGAL KEYS + REQUIRED FIELDS: foreach accepts only id/kind/in/as/goto; any other key is rejected. Missing `in` or missing `as` is an error; an `as` value that is not a valid slot name is rejected."
  - "DEFINITE ASSIGNMENT: foreach DEFINES `as` (declared, static class Untrusted) and USES `in`. `in` must be declared before the foreach (declare-before-use) and defined on every path to it (must-defined); a body step reading `as` sees it defined (the foreach's exit adds `as`). A duplicate `as` slot (already declared) is rejected; an undeclared `in` is rejected."
  - "AT MOST ONE FOREACH (no nesting, v1): a recipe with more than one foreach step is rejected with a clear error (the kernel Faults on a nested foreach; the parser refuses it before hashing). A single foreach whose body is well-formed is accepted."
  - "EXIT STILL REJECTED: `kind: exit` still returns ErrNotImplemented (only foreach is enabled). An unknown kind still errors."
  - "HASH + ROUND-TRIP: the semantic hash includes the foreach node's in/as (canonical form extended); ParseDraft returns warnings separately; the fixture recipes and existing grammar/linter behavior are unchanged (regression)."
constraints: package recipe at workspaces/stag/recipe (extends the existing parser/linter). Depends on the stag root (NodeForeach, Step) and the existing yaml-quarantined parser. This is the PARSER half of foreach; the kernel half (Eval) shipped in U28. No new dependency.
