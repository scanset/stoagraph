# 31 — Control-plane auth (spec)

Recorded 2026-07-11. **Status: BUILT + live-verified 2026-07-11.** Proven live against the real cluster:
`dispatch → POST /approve` = **401** while `approve → POST /approve` = **200** on a genuinely held prod action;
anonymous `POST /sessions` = 401; the full turnkey run (event → dispatch → bind → gate-served context → 7 gated
reads → prod restart escalated) still works with auth ON. Both modules + console green. The last v1 blocker.
Before this, every trusted surface was **dev-open**: no
authentication anywhere. That was tolerable when the orchestrator was private and everything ran on one laptop.
Planning/26 (all-OSS, Docker-packaged) means **strangers will `docker compose up` this product**, so an
unauthenticated control plane is now a shipping defect, not a backlog item.

## The threat model — what "dev-open" actually grants today

| Open endpoint | What an unauthenticated caller gets |
| --- | --- |
| `POST /api/approvals/{id}/approve` (stag-serve) | **Mints the ed25519 signed release** (`egress.SignApproval`). Approve any escalation. **The human-in-the-loop gate is decorative.** |
| `POST /sessions` (daemon :8091) | Binds a session to **any recipe they choose** → mints a session token. The "the agent cannot choose its own recipe" invariant collapses: pick the most permissive recipe and act through the gate. |
| `POST/DELETE /api/recipes` (stag-serve) | **Rewrite the policy.** Make everything `allow`. Total bypass, silently. |
| `POST/DELETE /api/routes\|mcp-servers\|providers` | Re-point a tool at a permissive recipe; swap the downstream; inject a context provider. |
| `POST /api/models` (harness :8090) | Replace/plant provider API keys. |
| `POST /api/dispatch\|run` (harness) | Run arbitrary agent loops on the operator's keys, against the real cluster. |
| `GET /api/log`, `/api/routes`, `/api/recipes` | Read the whole policy + audit trail. |

Three of these are **complete defeats of the product's core claim**, not mere info leaks.

## The load-bearing principle (this is what shapes the design)

> **The orchestrator must be able to BIND sessions and POLL approval status — but must NEVER be able to APPROVE.**

If the harness's credential could approve, a compromised or prompt-injected orchestrator could **self-approve its
own escalations**, and the human gate evaporates. The entire product thesis is that the gate is *separate from*
the orchestrator precisely so the orchestrator cannot authorize itself. A single shared "admin token" handed to
the harness would **quietly destroy that separation** — it would make the architecture a lie while every test
still passed. So: **roles, not one token.**

## Roles

| Role | May do | Held by |
| --- | --- | --- |
| **`approve`** | `POST /api/approvals/{id}/approve\|deny` — the signed release | **The human only** (console). **NEVER the harness.** |
| **`admin`** | Policy CRUD: recipes, routes, mcp-servers, providers (POST/DELETE) + all reads | The human/console (policy author) |
| **`dispatch`** | `POST /sessions` (daemon) · `GET /api/routes\|providers\|recipes` (catalog) · `GET /api/approvals[/{id}]` (**poll only**) | **The orchestrator** (harness-serve) |
| **`operator`** | harness-serve's own API: models, event-map, `POST /api/dispatch\|run` | The human/console |

`admin` is strictly more powerful than `approve` in practice (an admin can rewrite a recipe to auto-allow), so
the split is not a containment boundary between *humans* — it is least-privilege, and above all it is what keeps
**`dispatch` (the machine) strictly below both.** That is the boundary that matters.

## Endpoint → required role

**stag-serve :8080**
- `POST /api/approvals/{id}/approve|deny` → **`approve`**
- `POST|DELETE /api/recipes|routes|mcp-servers|providers`, `POST /api/recipes/validate` → **`admin`**
- `GET /api/recipes|routes|mcp-servers|providers|approvals|approvals/{id}|log|policies`, `POST /api/decide` → **any valid role** (`admin`, `approve`, or `dispatch`)
- `GET /api/health`, `GET /` (console assets) → **open**

**stag-proxy daemon :8091**
- `POST /sessions` → **`dispatch`**
- `/mcp/<token>` → **unchanged, no bearer.** The opaque per-session token in the path IS the credential; it is
  minted by the trusted binder and the agent is untrusted by design. Adding a bearer here would hand the agent a
  control-plane credential — exactly backwards.
- `GET /health` → **open**

**harness-serve :8090**
- `POST|DELETE /api/models`, `GET|POST /api/event-map`, `POST /api/dispatch|run` → **`operator`**
- `GET /`, `/logo.png` → **open** (console assets)

## Mechanism

- **`Authorization: Bearer <token>`**, compared with **`crypto/subtle.ConstantTimeCompare`** (no timing oracle).
- **Tokens file** — JSON, mode `0600`, gitignored: `deploy/mcp/control.tokens`
  `{"admin":"…","approve":"…","dispatch":"…","operator":"…"}`. 32 random bytes each, hex.
  Flag `-tokens <path>` on all three services.
- **stag-serve OWNS generation**: if the file is absent it generates all four, writes `0600`, and logs the path
  once (exactly the established pattern — `-approval-key` already auto-generates an ed25519 key). The daemon and
  harness-serve **read** the same file. This keeps a fresh `docker compose up` closed-by-default with zero setup.
- **Env overrides** (for containers/k8s secrets, take precedence over the file):
  `STAG_ADMIN_TOKEN`, `STAG_APPROVE_TOKEN`, `STAG_DISPATCH_TOKEN`, `HARNESS_OPERATOR_TOKEN`.
- **Fail closed:** a protected route with no configured token for its role → **401**, never open. A service that
  can neither read nor generate its tokens file → **refuses to start**.
- **`-dev-no-auth`** — an explicit escape hatch for the local loop/tests. Logs a **loud** warning on every start
  (`CONTROL PLANE UNAUTHENTICATED`). Never the default; never set in the compose file.
- **Audit:** log every 401 (method, path, remote addr). A security product should say when it was probed.

## Wiring the callers

- `dispatch.StagClient` + `dispatch.Binder` gain a `Token` field → send `Authorization: Bearer <dispatch>` on
  `GET /api/*` and `POST /sessions`.
- `agent.ApprovalConfig` (the poll loop) gains the **`dispatch`** token — it polls `GET /api/approvals/{id}`.
  **It must NOT receive `approve`.** (The retry replays with the *signed release token* the human's approval
  produced — that is a per-action ed25519 signature, not a control-plane credential. Unchanged.)
- **The console** sends `admin`/`approve` as bearer headers; the operator pastes them once (stored client-side).
  Real user identity (OIDC) is v2 — noted below.

## Build order + verification

1. **`auth` package (stag)** — load/generate the tokens file; env override; `Middleware(role)` + constant-time
   compare; `-dev-no-auth` bypass. Unit: correct token passes; wrong/absent → 401; a `dispatch` token is
   **rejected** on an `approve` route; constant-time compare used.
2. **stag-serve** — apply the role map above to its mux. Unit: table test asserting every route's required role,
   **especially that `dispatch` cannot reach `/approve`**.
3. **daemon** — `dispatch` on `POST /sessions`; `/mcp/<token>` untouched. Unit: bind without a bearer → 401;
   with `dispatch` → 200; the agent still connects to `/mcp/<token>` with no bearer.
4. **harness-serve** — `operator` on its API; thread the `dispatch` token into `StagClient`/`Binder`/
   `ApprovalConfig`.
5. **Console** — token entry + bearer on every call.
6. **Live re-verify the k8s turnkey run** end-to-end with auth ON (the whole loop must still work), then
   **negative tests**: unauthenticated `POST /sessions` → 401; harness's `dispatch` token on `/approve` → 401.

## Scope / not-in-v1

- **User identity / OIDC / SSO** — v2. v1 is shared-secret bearer tokens (correct for an operator tool; the
  console is not a multi-tenant app yet).
- **mTLS**, rate limiting, token rotation/expiry, per-token audit identity — v2.
- **Approval as a real signed human identity** (rather than "whoever holds the `approve` token") — v2, and the
  natural home for OIDC. Note the ed25519 *release* is already per-action and unforgeable; what v1 does not prove
  is **which human** approved.
