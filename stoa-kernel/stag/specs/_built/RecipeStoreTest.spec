name: RecipeStoreTest
role: test
intent: Verify recipe validation + persistence: a good recipe validates with a tier preview; a broken recipe reports the linter error; Save refuses to persist an invalid recipe (fail closed) and round-trips a valid one; Get/Delete reject path-traversal names; List validates all stored recipes. A fuzz drives arbitrary bytes through Validate (never panics; Valid iff ParseDraft succeeds) and through Save (never writes an invalid recipe).
api:
  - func TestValidateGood(t *testing.T)
  - func TestValidateBad(t *testing.T)
  - func TestSaveGetRoundTrip(t *testing.T)
  - func TestSaveFailsClosed(t *testing.T)
  - func TestNameSanitized(t *testing.T)
  - func TestList(t *testing.T)
  - func FuzzValidate(f *testing.F)
prelude: "A good recipe constant (a propose->authoritative-sink policy with a set_membership rule allowing a few labels). A broken recipe constant (e.g. an unquoted set member, or a missing actor on a ruled sink). Store tests use t.TempDir() as Dir."
behavior:
  - "VALIDATE GOOD: Validate(good) returns Valid true, Name the recipe name, a non-empty Hash, and Tiers with one row per allowed label whose Verdict is allow and Tier is auto (the authoritative sink emits a release event). No Error."
  - "VALIDATE BAD: Validate(broken) returns Valid false and a non-empty Error naming the problem; Name and Tiers are empty. Validate(nil) and Validate([]byte(\"\\xff\\x00 garbage\")) return Valid false without panicking."
  - "SAVE + GET ROUND-TRIP: with a Store over a temp dir, Save(good) returns Valid true and nil error and creates <Name>.yaml; Get(Name) returns bytes equal to good; List() returns one result with that Name."
  - "SAVE FAILS CLOSED: Save(broken) returns Valid false and a non-nil error, and writes NOTHING (the temp dir has no new file; a subsequent Get for any name fails). An invalid recipe is never persisted."
  - "NAME SANITIZED: Get and Delete reject names containing a slash, \"..\", or characters outside [a-z0-9_] (e.g. \"../../etc/passwd\", \"a/b\", \"\") with a non-nil error and no filesystem access outside Dir. Get of a valid-but-absent name is a non-nil error."
  - "LIST: after saving two valid recipes, List() returns two results sorted by name; an empty/absent Dir yields an empty list and nil error."
  - "FUZZ FuzzValidate(src []byte): Validate(src) never panics, and its Valid flag equals (recipe.ParseDraft(src) returns a nil error) - the same oracle. When Valid, Name is non-empty and Tiers is computed without panic. Additionally, Store.Save(src) to a temp dir writes a file IFF Validate(src).Valid (an invalid recipe is never written). Seed with the good recipe, the broken recipe, empty, and binary bytes."
constraints: package recipestore_test (external test); depends on the recipestore package, the recipe package (ParseDraft, for the fuzz oracle), and stdlib (os, path/filepath, testing, bytes). No network.
