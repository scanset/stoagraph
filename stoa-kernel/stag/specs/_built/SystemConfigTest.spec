name: SystemConfigTest
role: test
intent: Verify the system config loads a well-formed YAML with defaults applied, rejects every fail-closed case (unknown key, unknown enum, missing required, malformed, empty), keeps API-key resolution out of Load, and - adversarially - never panics and never leaks a partial config on error.
api:
  - func TestLoadOK(t *testing.T)
  - func TestLoadRejections(t *testing.T)
  - func TestLoadDefaults(t *testing.T)
  - func FuzzLoad(f *testing.F)
behavior:
  - "LOAD OK: a full config (proposer base_url+model+api_key_env, embedder base_url+model, kb docs+cache, egress jsonl+path, transport stdio) loads with a nil error and every field populated as written; APIKeyEnv holds the env var NAME (not its value); Load reads no environment. A second Load of the same bytes returns an equal Config (determinism)."
  - "DEFAULTS: a minimal config with only proposer {base_url, model} loads OK; the result has Proposer.Kind==\"openai\", Egress.Kind==\"memory\", Transport.Kind==\"stdio\" (defaults applied); Embedder and KB are zero (optional, not yet required)."
  - "REJECTIONS - each returns a non-nil error AND the zero Config: an unknown top-level key (e.g. proxy:); an unknown nested key (proposer.temperature:); malformed YAML; an empty document; a proposer missing base_url; a proposer missing model; proposer.kind: anthropic (unsupported); egress.kind: kafka (unknown); egress.kind: jsonl with no path; transport.kind: http (unsupported); an embedder with base_url but no model (inconsistent)."
  - "LOADFILE: LoadFile on a written temp file returns the same Config as Load of its bytes; LoadFile on a nonexistent path returns a non-nil error and the zero Config."
  - "FUZZ FuzzLoad: seed with the full config, the minimal config, empty, and a malformed snippet. Body takes []byte. Assert Load NEVER panics; if err==nil the Config is internally consistent (Proposer.BaseURL and Model non-empty, Proposer.Kind in the known set, Egress.Kind in {memory,stdout,jsonl}, Transport.Kind==\"stdio\") and a second Load is equal (determinism); if err!=nil the returned Config equals the zero Config (no partial leak)."
constraints: package config_test (external test); depends on the config package, os (for the temp file + asserting no env read), reflect, strings, testing.
