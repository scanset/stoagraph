# 28 — Downstream MCP server authentication (spec)

Recorded 2026-07-06. Companion to Planning/24 (stag-proxy), /26 (release), /27 (mcp nail-down). Lets `stag-proxy`,
as an MCP **client**, authenticate to a downstream MCP **server** that requires it. Two tiers, both specified:
**Tier 1 (static credential)** for v1, **Tier 2 (OAuth 2.1)** for v1.1.

**Status: Tier 1 BUILT + verified (2026-07-06).** `mcp_server` gained the auth columns (re-init done); `store`
resolves `Credential()` (env-ref-or-direct); `mcpgate` injects a bearer/header credential on the downstream HTTP
client via a RoundTripper (fail-closed: an empty-but-required credential errors before connect; `oauth` errors as
v1.1); the API masks the secret + preserves it on edit; the console Adapters form + list show auth (no secret).
Verified: httptest bearer/header-protected MCP server (right token connects; none/wrong/empty fail closed); live
storage + masking (`SECRET LEAKED: False`) + preserve-on-edit; 16 packages green. **Tier 2 (OAuth) remains v1.1.**

## Why (gap + feature)

- **Gap:** an HTTP downstream is dialed as `StreamableClientTransport{Endpoint: target}` with no credential, and
  `mcp_server` has no auth column. So today the gate can only front LOCAL stdio subprocesses or UNAUTHENTICATED
  HTTP servers. Anyone fronting a real remote MCP server (GitHub, Slack, a hosted DB tool) is blocked immediately.
- **Feature — credential isolation:** when the GATE holds the downstream credential, the agent never touches your
  GitHub/DB/SaaS token — the gate does, and it gates + audits every use of it. Same containment thesis as PII,
  applied to credentials. This is consistent with "the gate holds no keys": that principle is about MODEL/LLM keys
  (they stay in the orchestrator). Downstream SERVICE credentials are a different, necessary class — the gate is
  the MCP client to the service, so it is the correct holder.

**Scope note:** credential injection is HTTP-only. A stdio downstream (subprocess) authenticates via its own
process env — it inherits the proxy's environment, so `GITHUB_TOKEN` in the proxy env already reaches a spawned
stdio server. No code needed for stdio; `auth_scheme` there is `none`.

## Grounding: the go-sdk gives us both tiers as WIRING

`github.com/modelcontextprotocol/go-sdk@v1.6.1` already provides:
- `mcp.StreamableClientTransport.HTTPClient` — inject a custom `*http.Client` (Tier 1: a RoundTripper that adds a
  header).
- `mcp.StreamableClientTransport.OAuthHandler` (field) + the `auth/` package: `GetAuthServerMetadata` (RFC 8414
  discovery), `auth/authorization_code.go`, `auth/extauth/client_credentials.go`, and `oauthex` (metadata / dynamic
  client registration). Uses `golang.org/x/oauth2`. (Tier 2: set `OAuthHandler`, don't hand-roll.)

So neither tier is a from-scratch OAuth build.

## Data model (one DDL change — re-init required)

`mcp_server` gains auth config. Per the no-migrations rule, adding columns = edit `store/schema.sql` + **re-init**
`config.db` (re-add the servers; recollect is fine).

```
ALTER-free re-init. New mcp_server columns:
  auth_scheme  TEXT NOT NULL DEFAULT 'none',   -- none | bearer | header | oauth
  auth_header  TEXT NOT NULL DEFAULT '',        -- header scheme: the header name (bearer => Authorization)
  secret       TEXT NOT NULL DEFAULT '',        -- dev: the bearer/header value, or the oauth client_secret
  secret_env   TEXT NOT NULL DEFAULT '',        -- PREFERRED: env var holding the secret (not persisted)
  oauth_config TEXT NOT NULL DEFAULT '{}'        -- oauth non-secret JSON: {client_id, scopes[], token_url?}
```

`store.MCPServer` gains `AuthScheme, AuthHeader, Secret, SecretEnv, OAuthConfig` + a `Credential()` resolver
(`Secret` if set, else `os.Getenv(SecretEnv)`) — mirrors `event_harness store.Model.Key()`. The API NEVER echoes
`secret` (a masked hint, like the model KeyHint); `secret_env` is fine to show.

## Tier 1 — static credential (v1)

- **Injection:** for an HTTP downstream, `mcpgate` builds an `*http.Client` whose transport is an
  `authRoundTripper{header, value}` wrapping `http.DefaultTransport`: it clones the request and sets the header.
  - `bearer` → `Authorization: Bearer <credential>`
  - `header` → `<auth_header>: <credential>`
  - `none` → no client override (unchanged).
  Set it on `StreamableClientTransport.HTTPClient`.
- **Signature change:** `mcpgate.Connect` and `mcpgate.DiscoverTools` take an `Auth` struct (`{Scheme, Header,
  Credential string}`); callers (`cmd/stag-proxy` pickDownstream, `cmd/stag-serve` Discover) build it from
  `store.MCPServer` via `Credential()`. Both discovery AND the runtime connection authenticate.
- **Fail closed:** a scheme that needs a credential which resolves empty → the downstream connect FAILS (no
  downstream → its tools are absent → denied). Never silently connect unauthenticated.
- **Console/API:** the Adapters (mcp-servers) tab + `POST /api/mcp-servers` gain: scheme select, header name, and
  secret (env-var name preferred; direct value masked). Default `secret_env`.

## Tier 2 — OAuth 2.1 (v1.1)

- **Flow (MCP auth spec):** an unauthenticated request → `401` + `WWW-Authenticate` → protected-resource metadata
  (RFC 9728) → auth-server metadata (RFC 8414) → token. The go-sdk's `auth` package + `OAuthHandler` handle the
  dance + token cache/refresh.
- **Grant:** `client_credentials` (machine-to-machine) is the proxy's natural fit and the primary target — the
  proxy holds a `client_id` + `client_secret` registered with the downstream's auth server. `authorization_code`
  + PKCE (user-delegated) is a sub-case needing one-time human consent + refresh-token storage — later.
- **Config:** `auth_scheme='oauth'`, `oauth_config={client_id, scopes, token_url?}`, `secret_env` = the client
  secret. `token_url` optional (else discovered from the server).
- **Wiring:** set `StreamableClientTransport.OAuthHandler` to a handler built from `auth/extauth/client_credentials`.
- **Deferred to v1.1** — bigger (config + token lifecycle + testing), and not blocking the OSS v1.

## Security posture

- **Env-ref default** — persist `secret_env`, not the secret, in `config.db`. A direct `secret` is allowed in dev
  (unencrypted, consistent with our stance). Encryption-at-rest / a secrets backend = prod/v1.1.
- The API masks secrets; **credential isolation** is documented as a security property (the agent never holds the
  downstream credential; the gate mediates + audits every use).

## Verification plan

- **Tier 1:** stand up a bearer-protected HTTP MCP server (a minimal one, or the go-sdk `examples/auth/server`).
  Configure the credential (via `secret_env`). Confirm: correct token → discovery + a cleared `tools/call` forward;
  a wrong/missing token → the downstream connect fails closed (tools denied). Confirm the secret never appears in
  `GET /api/mcp-servers`.
- **Tier 2 (v1.1):** against the go-sdk OAuth example server with `client_credentials`.

## Open decisions (for review)
1. **Schema shape:** explicit columns (`auth_scheme/auth_header/secret/secret_env/oauth_config`) as above, vs a
   single `auth_config` JSON blob. Recommend explicit columns (clean secret-masking).
2. **Tier 1 header flexibility:** `bearer` + arbitrary `header` (recommended) — enough? (Some servers want a query
   param or a non-standard scheme; punt those to "custom" later.)
3. **Re-init acceptance:** the schema change requires a `config.db` re-init (re-add servers). OK for dev/v1?
4. **Secret storage:** env-ref default with direct allowed in dev — agree? Or env-ref only for the OSS build?

## Sequencing
1. **v1 (Tier 1):** schema + `store.MCPServer`/`Credential()` + `authRoundTripper` + `Connect`/`DiscoverTools` auth
   param + callers + console/API + verify. (Then re-init and re-add k8s-ops etc.)
2. **v1.1 (Tier 2):** OAuth via `OAuthHandler` (client_credentials first).
