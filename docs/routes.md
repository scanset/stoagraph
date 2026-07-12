# Routes — binding a tool to a policy

A **recipe** is a policy. A **route** is what makes that policy apply to a real tool. Nothing the gate
does is possible without one, and the most important thing about routes is what happens when there
isn't one.

```
route  =  tool  →  recipe  →  gateArg
          ^^^^     ^^^^^^     ^^^^^^^
          which    which      which argument(s) the
          tool     policy     policy actually judges
```

## The rule that matters: no route, no call

A tool with no route is **denied**. Not forwarded, not passed through, not "allowed because nobody said
otherwise". Denied.

```go
route, ok := g.Routes[call.Tool]
if !ok {
    // fail closed: a tool with no policy is denied, never forwarded.
    return Decision{Verdict: stag.Deny, Forward: false, Fault: "no recipe for tool " + call.Tool}
}
```

This is why **connecting a tool server grants the agent nothing**. Register GitHub's MCP server and the
agent can see 44 tools — `delete_file`, `create_repository`, `merge_pull_request` — and can call exactly
zero of them. Authority appears only where you wrote a route. Adding capability is an explicit,
auditable act; forgetting to add one fails safe.

## Creating a route

Console → **Adapters** → route a tool. Or:

```bash
curl -H "Authorization: Bearer $STAG_CONSOLE_TOKEN" -X POST localhost:8080/api/routes \
  -d '{"tool":"get_file_contents","recipe":"github_repo_policy","gateArg":"owner,repo"}'
```

A route whose recipe is missing or fails the linter is **rejected at build** (`router.Build` collects it
as an error and does not install it), so a typo'd recipe name cannot silently open a tool. `GET
/api/routes` reports each route's `valid` + `error`.

## gateArg — which arguments the policy sees

`gateArg` names the argument(s) the recipe judges. It is either a single name, or a **comma-separated
list**:

```json
{"tool":"notify",            "recipe":"notify_policy",     "gateArg":"channel"}
{"tool":"scale_deployment",  "recipe":"k8s_scale_policy",  "gateArg":"namespace,replicas"}
```

Each listed name binds the recipe's `propose out: <name>` slot of the same name, so one recipe can decide
from the whole action rather than one field of it:

```yaml
steps:
  - {id: propose_ns,   kind: propose, out: namespace}   # <- binds the `namespace` argument
  - {id: propose_reps, kind: propose, out: replicas}    # <- binds the `replicas` argument
```

Two behaviours worth knowing:

- **A missing argument binds `""`** and therefore fails its rule. An agent cannot dodge a gate by simply
  omitting the argument.
- Arguments you do **not** list are **not judged**. They are still forwarded to the tool. This is the
  trap below.

## The trap: gate every argument that changes the blast radius

Gating the obvious argument is not the same as gating the action. A real example:

```yaml
# WRONG — gates the repo, ignores the owner
rules:
  repo.allowed: {kind: set_membership, set: ["stoagraph"]}
```
```json
{"tool":"get_file_contents","recipe":"github_repo_policy","gateArg":"repo"}
```

Intent: *"the agent may read our repo."* Actual effect:

| call | verdict |
|---|---|
| `get_file_contents(owner=scanset, repo=stoagraph)` | ALLOW ✅ |
| `get_file_contents(owner=**mallory**, repo=stoagraph)` | **ALLOW** ❌ |

`owner` was never judged, so *anybody's* repo named `stoagraph` passes. The fix is a route that binds
both arguments, and a recipe that sinks both — every sink must clear for the call to be allowed:

```yaml
rules:
  owner.allowed: {kind: set_membership, set: ["scanset"]}
  repo.allowed:  {kind: set_membership, set: ["stoagraph"]}
steps:
  - {id: propose_owner, kind: propose, out: owner}
  - {id: propose_repo,  kind: propose, out: repo}
  - {id: check_owner, kind: sink, in: owner, field: github.owner, sensitivity: authoritative, rule: owner.allowed, actor: "policy:github"}
  - {id: check_repo,  kind: sink, in: repo,  field: github.repo,  sensitivity: authoritative, rule: repo.allowed,  actor: "policy:github"}
```
```json
{"tool":"get_file_contents","recipe":"github_repo_policy","gateArg":"owner,repo"}
```

Now `mallory/stoagraph` and `scanset/secret-repo` are both denied.

**When you write a route, ask: which arguments, if changed, would change who or what this touches?**
Every one of those belongs in `gateArg`.

## The approval token

`approval_token` is a **gate-only meta argument**. A recipe that escalates to a human uses a
`signed_equality` rule against `"$approved"`; on the approved retry the orchestrator attaches
`approval_token`, and the route must list it so the gate can bind it:

```json
{"tool":"scale_deployment","recipe":"k8s_scale_approval_policy","gateArg":"namespace,replicas,approval_token"}
```

It never appears in the audit record's value — the gate strips it, because it is a credential, not a fact
about the action.

## What lands in the record

**Every decision is recorded** — allowed, denied, and escalated alike. A blocked attempt is the evidence
the control worked, and a log of only the permitted actions cannot answer the question an auditor
actually asks: *did anything try?* An unrouted tool call is recorded too — reaching for a capability that
was never granted is the most suspicious call of all.

Each leaf carries the tool, the verdict, whether it was **forwarded**, the bound value, and the recipe.
The audit value depends on the shape of the route:

- **single-arg** — the raw value: `tmpl:account_unlocked`
- **multi-arg** — the bound pairs: `owner=scanset repo=stoagraph`

A leaf also carries its **releases** — the crossings where an untrusted value actually reached an
authoritative sink. Releases appear **only on a forwarded call**. This matters precisely because of the
multi-arg trap above: a denied call can still contain a sink that individually cleared (`owner=mallory`
fails while `repo=stoagraph` passes), and recording that as a release would put a crossing in the
tamper-evident log **that never happened** — the audit would assert the agent read a repo the gate
actually blocked. The record states what *happened*, never what merely *evaluated*.

## Global routes vs session routes

There are two ways routes reach the gate, and they are not the same thing:

- **The global route table** (`/api/routes`, stored in `config.db`) — what the console manages, and what a
  stdio gate uses.
- **Session routes** — in daemon mode the trusted dispatcher binds a session with its own route set:
  `POST /sessions {routes:[{tool,recipe,gateArg}]}`. The agent connects to `/mcp/<token>` and gets
  **only** those routes. The agent cannot choose its own recipe, and a session with no routes binds
  nothing. See [mcp-gating-proxy.md](mcp-gating-proxy.md).

## See also

- [recipe-authoring.md](recipe-authoring.md) — writing the policy a route points at
- [mcp-gating-proxy.md](mcp-gating-proxy.md) — how a cleared call is forwarded, and the session model
