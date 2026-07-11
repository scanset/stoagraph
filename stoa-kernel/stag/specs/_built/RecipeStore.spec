name: RecipeStore
role: component
intent: The recipe-authoring core for the admin console (Planning/16, recipe-authoring slice) - validate recipe YAML through the REAL linter and persist valid recipes. Validate(src) runs recipe.ParseDraft and returns a ValidateResult: whether it is valid, the recipe name + semantic hash, the parse/lint error (if invalid), the draft warnings, and a TIER PREVIEW (each label in the recipe's rule sets evaluated through the kernel to its verdict + tier - auto/escalate/benign/deny). A file-backed Store persists recipes by name: Save validates first and REFUSES to write an invalid recipe (fail closed); List/Get/Delete round out CRUD. The recipe NAME comes from the parsed recipe (grammar-sanitized: lowercase/digits/_, max 64), so it is safe as a filename - no path traversal; Get/Delete additionally reject any name that is not a valid recipe identifier.
api:
  - "type TierRow struct { Label string; Verdict string; Tier string }"
  - "type ValidateResult struct { Valid bool; Name string; Hash string; Error string; Warnings []string; Tiers []TierRow }"
  - func Validate(src []byte) ValidateResult
  - "type Store struct { Dir string }"
  - func (s Store) List() ([]ValidateResult, error)
  - func (s Store) Get(name string) ([]byte, error)
  - func (s Store) Save(src []byte) (ValidateResult, error)
  - func (s Store) Delete(name string) error
concept: validate recipe YAML via the real parser+linter; a tier preview per label; file-backed CRUD; fail-closed save (never persist an invalid recipe); grammar-sanitized names (no traversal).
behavior:
  - "VALIDATE: Validate(src) calls recipe.ParseDraft(src). If it errors, ValidateResult{Valid:false, Error: the error} (Name/Hash/Tiers empty). If it parses, Valid:true, Name = the recipe name, Hash = the semantic hash, Warnings = the draft warnings, and Tiers = one TierRow per label in the union of the recipe's rule sets: Label, Verdict = stag.Eval(recipe, label, hash).Verdict.String(), Tier = auto (Allow with a release event) | benign (Allow, no event) | escalate (Escalate) | deny (otherwise). Validate NEVER panics on any input (malformed YAML, huge input, binary)."
  - "SAVE FAILS CLOSED: Save(src) first Validates. If invalid, it returns the ValidateResult and a non-nil error and WRITES NOTHING. If valid, it creates Dir if needed and writes src verbatim to Dir/<Name>.yaml (Name is grammar-sanitized by the parser, so it cannot escape Dir), returning the ValidateResult and nil. Saving an existing name overwrites it (edit)."
  - "GET: Get(name) returns the raw bytes of Dir/<name>.yaml. It rejects (non-nil error, no read) any name that is not a valid recipe identifier (contains a slash, dot-dot, or a character outside [a-z0-9_], or is empty/over 64 chars) - no path traversal. A missing recipe is a non-nil error."
  - "DELETE: Delete(name) removes Dir/<name>.yaml, rejecting an invalid name the same way as Get. Deleting a missing recipe is a non-nil error."
  - "LIST: List() reads every *.yaml in Dir, runs Validate on each, and returns the results sorted by name. A missing Dir yields an empty list and a nil error (an empty store is not an error). An unreadable individual file is skipped."
  - "PURE/DETERMINISTIC VALIDATE: Validate performs no I/O and is deterministic (the parser + kernel are). The Store's methods touch only files under Dir."
constraints: package recipestore at workspaces/stag/recipestore (public; import path github.com/scanset/StAG/recipestore). Depends on the recipe package (ParseDraft, Parsed.Rules), the stag root (Eval, Verdict, EvalResult), and stdlib (os, path/filepath, sort, strings, fmt). No network, no MCP dependency.
