name: ModelProvider
role: component
intent: The model-provider config surface — connect Claude (Anthropic Messages API) and OpenRouter/OpenAI-compatible models from the console (Planning/21). A store table + typed CRUD + serve endpoints. The store NEVER holds an API key: it stores `api_key_env`, the NAME of an environment variable; the server resolves the key with os.Getenv at call time and reports only whether it is SET (keyPresent), never the value. Config + management only — this unit does NOT invoke a model (no live prompts).
api:
  - "store: model_provider table {name PK, kind, base_url, model, api_key_env, enabled}; ONE DDL, no migrations."
  - "store: type ModelProvider{Name,Kind,BaseURL,Model,APIKeyEnv,Enabled}; PutModelProvider (upsert), ListModelProviders (ordered), DeleteModelProvider, GetModelProvider (fail-closed not-found)."
  - "serve: GET/POST/DELETE /api/models; ModelProviderView adds keyPresent bool (os.Getenv(api_key_env) != \"\") and NEVER echoes the key value."
concept: connect a model by dialect+endpoint+model+key-env; the DB records which env var holds the key, the server reads it at runtime; the console shows configured/missing without exposing the secret.
behavior:
  - "STORE CRUD: PutModelProvider upserts by name; ListModelProviders returns all ordered by name; GetModelProvider returns the row or a fail-closed error for an unknown name; DeleteModelProvider removes it. All queries parameterized (arbitrary strings inert). enabled round-trips."
  - "NO SECRET AT REST: the schema has api_key_env (a variable NAME) and NO key column. A ModelProvider carries no secret; the JSON view never contains a key. The only key-derived output is keyPresent, computed from os.Getenv at request time."
  - "SERVE LIST/PUT/DELETE: GET /api/models lists views (name, kind, base_url, model, api_key_env, enabled, keyPresent). POST validates kind is claude|openai, name/model/api_key_env non-empty, and base_url present for openai; upserts; returns the view. DELETE removes by path name. No store -> empty list / 501 on write (matches providers)."
  - "KIND VALIDATION: kind must be claude or openai; any other is rejected 400. base_url is required for openai (endpoint) and optional for claude (SDK default). model and api_key_env are required for both."
  - "FAIL CLOSED + REGRESSION: a malformed body is 400; a store error is 500; deleting an absent provider is not a hard error. The existing adapters (mcp servers, context providers, routes) are unchanged; re-init after the DDL edit recreates all tables."
constraints: package store (schema.sql + store.go, modernc quarantined) + package serve (models.go, SDK-free — config only, no model adapter imported here). Mirrors the context_provider slice (U25/U27). This is the CONFIG surface; building a live model.Proposer from a stored provider (key from env) and wiring it to the gate is a later unit. No new dependency.
