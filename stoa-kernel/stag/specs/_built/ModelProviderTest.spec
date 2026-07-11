name: ModelProviderTest
role: test
intent: Verify the model-provider config surface — store CRUD round-trips, no secret is ever stored or returned, the serve endpoints validate kind and required fields, and keyPresent reflects the environment without exposing the value. Includes a fuzz target over the store CRUD.
api:
  - func TestModelProviderStoreCRUD(t *testing.T)
  - func TestModelProviderNoSecretColumn(t *testing.T)
  - func TestModelProviderServeValidation(t *testing.T)
  - func TestModelProviderKeyPresent(t *testing.T)
  - func FuzzModelProviderStore(f *testing.F)
prelude: "A temp SQLite store (store.Open on a temp path). A serve.Server with that store. Providers: a claude one (api_key_env ANTHROPIC_API_KEY, blank base_url) and an openai one (base_url https://openrouter.ai/api/v1, api_key_env OPENROUTER_API_KEY)."
behavior:
  - "STORE CRUD: Put two providers; List returns both ordered by name with fields intact (kind, base_url, model, api_key_env, enabled); Get returns one and errors on an unknown name; Delete removes one and List reflects it. A second Put on the same name updates in place (upsert)."
  - "NO SECRET: the ModelProvider struct and the persisted row carry api_key_env but no key field; a value that looks like a secret set as api_key_env is stored verbatim as a NAME (not resolved), and no column holds a key."
  - "SERVE VALIDATION: POST /api/models with kind not in {claude,openai} -> 400; missing model or api_key_env -> 400; openai with blank base_url -> 400; a valid claude/openai body -> 200 and the view (no key value present in the JSON). GET returns the list; DELETE removes by name."
  - "KEY PRESENT: with an env var set (t.Setenv), the view's keyPresent is true; with the referenced var unset, keyPresent is false; the actual value never appears in the response."
  - "FUZZ (FuzzModelProviderStore): for arbitrary name/kind/base_url/model/api_key_env/enabled, PutModelProvider then GetModelProvider round-trips the exact fields (or fails closed), never panics, and the DB never yields a key value that was not stored as api_key_env."
constraints: package store (internal test) + package serve (httptest over the mux). Depends on store, serve, net/http/httptest, testing. The fuzz target is deterministic and bounded (temp DB per run or shared with unique names)."
