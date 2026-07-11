name: SystemConfig
role: component
intent: The StoaGraph SYSTEM config (runtime, unit 1) - how a running instance is wired to its infrastructure, NOT the policy it enforces. It configures the proposer endpoint (the chat model, e.g. ollama), the embedder endpoint (the embed model for the KB), the knowledge base location, the egress destination, and the transport. The task/deployment layer - the recipe (the signed gate), the trusted instruction prompt, and the sink->actuator bindings - is SEPARATE and loaded elsewhere; the config never carries the security-critical gate or the untrusted prompt. Operator-trusted input, so it does not need the recipe parser's full threat treatment, but it is still FAIL CLOSED: malformed YAML, unknown keys, an unknown enum, or a missing required field is a terminal error, never a silent default that mis-wires the system. Load is a pure function of bytes (no env reads, no I/O) so it is deterministic and testable; API-key resolution (reading the named env var) happens at wiring time, not here. Basic minimum now; embedder/KB are optional until the RAG unit consumes them.
api:
  - "type Proposer struct { Kind string; BaseURL string; Model string; APIKeyEnv string }"
  - "type Embedder struct { BaseURL string; Model string }"
  - "type KB struct { Docs string; Cache string }"
  - "type Egress struct { Kind string; Path string }"
  - "type Transport struct { Kind string }"
  - "type Config struct { Proposer Proposer; Embedder Embedder; KB KB; Egress Egress; Transport Transport }"
  - func Load(src []byte) (Config, error)
  - func LoadFile(path string) (Config, error)
behavior:
  - "PARSE FAIL CLOSED: Load decodes YAML into Config with unknown-key rejection (yaml Decoder KnownFields(true)) - a typo'd or unrecognized key is a terminal error, not ignored. Undecodable YAML returns (Config{}, error). An empty/whitespace document returns (Config{}, error) (a config with no proposer cannot wire anything)."
  - "PROPOSER (required): Proposer.BaseURL and Proposer.Model must be non-empty, else error. Proposer.Kind defaults to \"openai\" when empty; any other value errors (only the OpenAI-compatible dialect is supported now). Proposer.APIKeyEnv is the NAME of the env var holding the key (may be empty for keyless backends like ollama); Load does NOT read the environment - resolution is the runtime's job."
  - "EGRESS: Egress.Kind defaults to \"memory\" when empty; the allowed set is {memory, stdout, jsonl}; any other value errors. Kind \"jsonl\" requires a non-empty Path, else error; for memory/stdout, Path is ignored."
  - "TRANSPORT: Transport.Kind defaults to \"stdio\" when empty; any other value errors (only stdio is supported now)."
  - "EMBEDDER + KB (optional in v1, validated for consistency): if either Embedder field is set, BOTH BaseURL and Model are required, else error. If KB.Docs is set it is the runbook directory; KB.Cache is optional (an empty Cache means no persistent embedding cache). These are consumed by the RAG unit later; an absent embedder/KB is not an error yet."
  - "NORMALIZED RESULT: on success Load returns a Config with all defaults applied (Proposer.Kind, Egress.Kind, Transport.Kind filled), so a caller reads concrete values, never empty-means-default. Load is DETERMINISTIC: identical bytes yield an identical Config."
  - "LOADFILE: LoadFile reads the file at path and delegates to Load; a read error (missing/unreadable file) returns (Config{}, error). It performs no other logic."
  - "NEVER PANICS: Load returns an error for ANY input (fuzz-proven); it never panics, and a rejected input returns the zero Config (no partially-filled config escapes an error)."
constraints: package config at workspaces/stag/config (public; import path github.com/scanset/StAG/config). Depends on go.yaml.in/yaml/v3 (already a module dependency) and stdlib (bytes, fmt, os, io). No dependency on the kernel/model/broker packages - config is standalone infrastructure.
