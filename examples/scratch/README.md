# scratch

Throwaway recipes from development — minimal shapes used to exercise the linter and the verdict
tiers (`my_policy` allows the literal value `safe_value`). They are **not** meant as policy you would
deploy. Kept because they are useful, tiny worked examples of the grammar.

For real policy, see `examples/k8s/recipes/` (a live cluster: reads auto-allow, dev/staging mutate
freely, prod escalates, delete-namespace is denied outright).
