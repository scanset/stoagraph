# Docker

```bash
tools/gen-env.sh      # mint the four control-plane role secrets into .env (0600, gitignored)
docker compose up -d  # five containers
open http://localhost:3000
```

Five containers, two Dockerfiles:

| Service | Port | Image |
|---|---|---|
| `stag-serve` | 8080 | the **gate's** control plane — policy, approvals, audit |
| `stag-proxy` | 8091 | the **gate's** MCP proxy — sessions bound to a recipe |
| `harness-serve` | 8090 | the **orchestrator** — holds the model API keys |
| `kbserve` | 8095 | example context provider (the READ channel's downstream) |
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

## Registering a downstream (the one thing containers change)

**The bundled examples' MCP servers speak `stdio`** — the gate *spawns* them as a subprocess. That
works when you run the gate on your host (`tools/up.sh`), but a containerized gate cannot spawn them
without baking `python3` + `kubectl` into the gate image, which would make the gate fat and couple it
to one example. We deliberately did not do that.

So under Docker, register an **`http`** downstream instead — the gate supports it natively (it becomes
an MCP client over streamable HTTP):

```bash
ADMIN=$(grep STAG_ADMIN_TOKEN .env | cut -d= -f2)
curl -s -H "Authorization: Bearer $ADMIN" -X POST localhost:8080/api/mcp-servers \
  -d '{"name":"my-tools","transport":"http","target":"http://my-tools:9000/mcp"}'
```

`stag-proxy` picks it up on its next poll and flips to `ready`.

**Known gap:** the k8s / pii-demo / zt-ops example servers are stdio-only today, so the *containerized*
stack has no turnkey demo downstream yet. Run those examples with `tools/up.sh` on the host, or port an
example server to streamable HTTP. That port is the next piece of work, and it is tracked honestly here
rather than papered over.

## Operational notes

- `data/` is a named volume (config DB, the recipe store, audit logs). `docker compose down -v` wipes
  it back to a fresh instance.
- Every service answers `GET /health` unauthenticated, so one healthcheck shape works for all of them.
- Rotate the control plane: delete `.env`, re-run `tools/gen-env.sh`, `docker compose up -d`.
