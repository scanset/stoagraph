# Docker

```bash
stoagraph up      # mints the four role secrets, pulls the signed images, starts
stoagraph demo    # loads the containment demo
```

`compose.yml` is **pull-only** — it references published images and nothing on your disk, so it works
for someone who never cloned the repo. In a clone, `compose.override.yml` is merged automatically and
adds the `build:` blocks, so `docker compose build` still builds from source.

Doing it by hand:

```bash
tools/gen-env.sh && docker compose up -d && tools/demo.sh
```

## Why there is no `docker run` one-liner

The control plane uses **per-role secrets**, and `approve` — the token that releases a held action —
must never reach the orchestrator's environment. Something has to mint four distinct secrets and give
each service only what it is entitled to, *before anything starts*. That is the step a single
`docker run` cannot do, and it is precisely the thing we made impossible to shortcut. `stoagraph up`
does it and gets out of the way.

Six containers, two Dockerfiles:

| Service | Port | Image |
|---|---|---|
| `stag-serve` | 8080 | the **gate's** control plane — policy, approvals, audit |
| `stag-proxy` | 8091 | the **gate's** MCP proxy — sessions bound to a recipe |
| `harness-serve` | 8090 | the **orchestrator** — holds the model API keys |
| `kbserve` | 8095 | example context provider (the READ channel's downstream) |
| `pii-demo` | 9000 | an example tool server (streamable HTTP) — the containment demo |
| `console` | 3000 | one UI, two backends |

The Go services all come from **one** `Dockerfile` (`--build-arg CMD=<binary>`), on
`distroless/static` as `nonroot`: no shell, no package manager, ~25–43 MB. Nothing to pivot to if a
binary is ever popped — which is also why the healthcheck is a 3 MB static probe (`cmd/healthcheck`)
rather than dropping `curl` into the image.

## Why not one container

Because the secrets would all end up on one filesystem, and that **structurally defeats the
human-in-the-loop guarantee.**

The control plane uses per-role secrets, and `approve` — the one that releases a held action — belongs
to a human. If the orchestrator can reach it, a compromised orchestrator approves its own escalations.
And the gate's HTTP role check does **not** save you there: a compromised orchestrator would not send
`dispatch` to an approve route and politely accept the 401. It would send the `approve` token it was
holding.

So no container mounts the tokens file, and each service is injected only what it is entitled to. The
`environment:` blocks in `compose.yml` **are** the access-control matrix:

| Service | Secrets it receives |
|---|---|
| `stag-serve` | `admin`, `approve`, `dispatch` — it verifies all three |
| `stag-proxy` | `dispatch` — it only guards `POST /sessions` |
| **`harness-serve`** | **`dispatch`, `operator` — never `approve`.** Not "unused": *absent*. |
| `kbserve` | none |
| `console` | none — the human types their token into the browser |

Verify it yourself:

```bash
docker inspect stoagraph-harness-serve-1 --format '{{range .Config.Env}}{{println .}}{{end}}' \
  | grep -c "$(grep STAG_APPROVE_TOKEN .env | cut -d= -f2)"    # => 0
```

## Model API keys

**Preferred:** keep keys out of every file. Set `"apiKeyEnv": "ANTHROPIC_API_KEY"` in
`config/models.json` and put the key in `.env`; compose passes it to the orchestrator only. A mounted
secrets file is precisely the thing this design avoids.

If you do keep a key inside `config/models.json`, note the bind mount preserves host ownership: a
`0600` file is unreadable by the container's default user. `compose.yml` therefore runs
`harness-serve` as `${HOST_UID}` (written by `tools/gen-env.sh`).

Either way, **the gate never sees any of it.** `config/` is mounted into the orchestrator alone.

## A fresh instance is empty

No recipes, no routes, nothing pre-trusted. `stag-proxy` comes up **live but not ready** and says so:

```
GET  /health    {"ok":true,"ready":false}
POST /sessions  503 {"error":"gate not ready: no downstream MCP server connected yet"}
```

That is fail-closed, not a fault. A gate with nothing to mediate must not pretend it is mediating. It
polls for a downstream and starts serving the moment one is registered — no restart needed.

## The demo

```bash
tools/demo.sh
```

That authors the policy, registers the `pii-demo` tool server, and routes the tools. `stag-proxy` picks
the downstream up on its next poll and flips to `ready`. Then, with **no model and no API key**:

```
fetch_user_profile(123)                          ALLOW  — returns Alice's record, INCLUDING her SSN
send_external_reply("Your SSN is 000-12-3456")   DENY   — never reaches the tool
send_external_reply("Hi Alice, you're unlocked") DENY   — still free-form
send_external_reply("tmpl:account_unlocked")     ALLOW  — an approved template
```

Look at the third line. A perfectly innocent message is *also* blocked — because **no free-form value
can cross at all.** The policy does not scan for SSNs; it permits four template ids. That is why a
jailbroken or prompt-injected model cannot defeat it: there is no clever phrasing that becomes an
approved template id. **Containment is structural, not content-based.**

## Registering your own downstream

A containerized gate cannot *spawn* a `stdio` MCP server — and we would rather keep the gate distroless
and minimal than bake an interpreter into it. So under Docker, register an **`http`** downstream; the
gate is an MCP client over streamable HTTP natively:

```bash
ADMIN=$(grep STAG_ADMIN_TOKEN .env | cut -d= -f2)
curl -s -H "Authorization: Bearer $ADMIN" -X POST localhost:8080/api/mcp-servers \
  -d '{"name":"my-tools","transport":"http","target":"http://my-tools:9000/mcp"}'
```

`cmd/example-pii` is a 60-line reference for what such a server looks like (it serves stdio *and*
streamable HTTP from the same binary).

The **k8s example** (a live cluster: gated reads, prod mutations that escalate to a human) still runs
`stdio` and needs a real cluster, so drive it on the host: `tools/up.sh` then `bash
examples/k8s/setup.sh`.

## Operational notes

- `data/` is a named volume (config DB, the recipe store, audit logs). `docker compose down -v` wipes
  it back to a fresh instance.
- Every service answers `GET /health` unauthenticated, so one healthcheck shape works for all of them.
- Rotate the control plane: delete `.env`, re-run `tools/gen-env.sh`, `docker compose up -d`.
