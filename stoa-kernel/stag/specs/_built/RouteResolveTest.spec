name: RouteResolveTest
role: test
intent: Verify route resolution: valid bindings become router entries with the parsed recipe; a missing or invalid recipe leaves the tool unrouted and reported (fail closed), without poisoning valid siblings. A fuzz drives arbitrary recipe bytes and asserts a tool is in the router IFF its recipe parses, and every unresolved spec is reported.
api:
  - func TestBuildValid(t *testing.T)
  - func TestBuildFailsClosed(t *testing.T)
  - func FuzzBuild(f *testing.F)
prelude: "A valid policy recipe constant (propose -> authoritative sink with a set_membership rule). A loader helper backs recipes with an in-memory map[name][]byte; an absent name returns an error."
behavior:
  - "BUILD VALID: two specs {write_note->policyA, scale->policyB} with a loader holding both valid recipes. Build returns a Router of length 2; Router[\"write_note\"].GateArg matches the spec, and Router[\"write_note\"].RecipeHash is non-empty (the parsed hash). Errors is empty."
  - "BUILD FAILS CLOSED: three specs - one valid, one whose recipe name is ABSENT from the loader, one whose recipe bytes are garbage. Build returns a Router of length 1 (only the valid tool), and Errors of length 2 naming the absent and the garbage tools. The valid tool is unaffected. No Router entry exists for the failed tools."
  - "FUZZ FuzzBuild(src []byte): one spec {Tool:\"t\", Recipe:\"r\", GateArg:\"a\"} with a loader returning src for \"r\" and an error otherwise. Build. ASSERT: exactly one of (Router has \"t\") / (Errors has \"t\") is true; if \"t\" is routed then recipe.Parse(src) returns nil error AND Router[\"t\"].GateArg == \"a\"; if \"t\" is errored then recipe.Parse(src) returns a non-nil error; never panics; a second Build is equal. Seed with the valid recipe, empty bytes, and garbage."
constraints: package router_test (external test); depends on the router package, the recipe package (Parse, for the oracle), and stdlib (reflect, testing). No store/DB dependency.
