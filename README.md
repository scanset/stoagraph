# StoaGraph

**Verifiable control for AI agents.** An agent proposes; a deterministic gate disposes.

StoaGraph sits between an AI agent and the world. Every tool call the agent makes, and every piece of
context it reads, crosses a gate that decides — **with no model in the decision path** — whether it may
happen. Cleared calls are forwarded to the real tool; denied ones never reach it. Every crossing is
written to a tamper-evident log.

> The model has a flashlight. The gate has the map.

The point is not that the model behaves. The point is that **it doesn't have to.** A hijacked,
prompt-injected, or simply wrong model can propose anything it likes; it cannot make the gate release a
value the policy rejects.

## The two pieces

| | | |
|---|---|---|
| **`stag`** | the **gate** | Deterministic kernel, MCP gating proxy, policy, audit, approvals. **Holds no model and no API keys.** |
| **`harness`** | the **orchestrator** | Dispatcher, agent loop, model connections. **Holds the keys.** |

That separation is the product. It is also *enforced*, not merely intended:
[`architecture_test.go`](stoa-kernel/architecture_test.go) fails the build if any gate package — or
either gate binary — so much as imports orchestrator code. The gate can be trusted with your
infrastructure precisely because it is provably incapable of reaching your keys.

Everything here is open source under **[Apache-2.0](LICENSE)** — patent grant included, which is what
enterprise and public-sector legal teams will ask about first. There is no held-back edition.

## Both channels cross the gate

An agent can do two things: **act** and **read**. Both are mediated.

| Channel | MCP surface | What the gate does |
|---|---|---|
| **ACT** | `tools/call` | **allow / deny / escalate.** Forward-iff-cleared — a denied call never reaches the tool. |
| **READ** | `resources/read` | **label + record.** Context is stamped **untrusted at origin**, unbypassably, and the read is audited. |

Reads are never denied; they are *labeled*. Which brings us to the part most systems get wrong.

## The trust invariant (read this before trusting anything)

The untrusted stamp on context is **positional, not taint-tracking.**

An LLM launders taint. Untrusted text goes in, a tool call comes out, and there is no reliable way to
know which output bytes came from which input. Any system claiming to propagate a taint label *through*
a model is lying to you. StoaGraph does not try. Instead:

- **Into the model** — the label's only job is *placement*: untrusted context goes in the input slot,
  never the instruction slot, so it cannot rewrite the agent's goal.
- **Out of the model** — there is **no carried label at all.** Every proposal is *presumed untrusted*.
  The gate re-derives trust **at the sink**, from the policy rule — never from anything the model said.

The only promotions from untrusted to cleared are a rule firing (`set_membership`, `numeric_range`,
`signed_equality`), and each emits a recorded release event. So poisoned context can change *what the
agent proposes*. It cannot make the gate *release* a value the rule rejects. A bad read wastes a turn;
it does not breach.

## Humans stay in the loop — and the machine cannot forge that

When policy says an action needs a person, the gate **escalates** and holds the call. A human approves,
which mints an ed25519 **signed release** bound to that exact action, and only then does the call proceed.

The orchestrator can *bind sessions* and *poll* for the decision. It **cannot approve** — that is a
separate credential it is never given. An orchestrator able to approve its own escalations would make
the human gate decorative, and every test would still pass. So the roles are separate secrets and the
gate enforces the split. See [SECURITY.md](SECURITY.md).

## Install

```bash
curl -sSL https://raw.githubusercontent.com/scanset/stoagraph/v0.1.0/install.sh | sh
stoagraph up      # mints your control-plane role secrets, pulls the signed images, starts
stoagraph demo    # loads the containment demo — no model, no API key
```

Piping a script into your shell, from a product that says *"don't trust — verify"*? Fair. So:
`install.sh` lives **in this repo at the tag it installs** (what you read is what runs), it
**verifies the SHA-256** of the binary against published, cosign-signed checksums before executing
anything, and it prints the `cosign verify` commands for the images. Read it first if you like —
we would:

```bash
curl -sSLO https://raw.githubusercontent.com/scanset/stoagraph/v0.1.0/install.sh
less install.sh && sh install.sh
```

Already have Go? `go install github.com/scanset/stoagraph/stoa-kernel/cmd/stoagraph@latest`

<details><summary>From source</summary>

```bash
tools/gen-env.sh && docker compose up -d && tools/demo.sh
```
</details>

**No model or API key required to see the point:**

```
fetch_user_profile(123)                          ALLOW  — returns Alice's record, INCLUDING her SSN
send_external_reply("Your SSN is 000-12-3456")   DENY   — never reaches the tool
send_external_reply("Hi Alice, you're unlocked") DENY   — still free-form
send_external_reply("tmpl:account_unlocked")     ALLOW  — an approved template
```

Look at the third line. An *innocent* message is blocked too — because **no free-form value can cross
at all.** The policy never scans for SSNs; it permits four template ids. There is no clever phrasing
that becomes an approved template id, which is exactly why a jailbroken model cannot get around it.

Or run it on the host:

```bash
tools/build.sh && tools/up.sh     # prints your control-plane tokens
```

**Five containers, not one** — and that is the security posture, not a diagram. The `approve` secret
(the one that releases a held action) is injected into the gate and **never into the orchestrator**, so
a compromised orchestrator cannot approve its own escalation. One container would put every secret on
one filesystem and make that impossible to prevent. See [docs/docker.md](docs/docker.md).

**A fresh instance starts empty.** No recipes, no routes, nothing pre-trusted — a security control
must not arrive already permitting something you never authored. You load policy by running an example
(`tools/demo.sh`) or by authoring it in the console. The gate creates its own `data/` on first boot.

`tools/up.sh` prints four tokens. Paste the **gate** token (`admin`) and the **orchestrator** token
(`operator`) into the console sidebar. Keep `approve` for when you mean it — it is the one that releases
a held action.

To connect a model, copy `config/models.example.json` to `config/models.json` and add a key. **The gate
never sees it.**

Then drive the real demo against a live Kubernetes cluster:

```bash
tools/demo.sh             # wires the k8s example; tells you how to fire an incident
```

An incident event arrives → the dispatcher picks a policy → a session is bound **on the gate** → the
agent reads infra facts as untrusted context → investigates with gated reads → proposes a fix to
**prod** → the gate **escalates** and waits for you.

## Layout

```
stoa-kernel/   one Go module — the whole backend
  stag/        the GATE         (kernel, policy, proxy, auth, audit, approvals)
  harness/     the ORCHESTRATOR (dispatch, agent loop, models)
  cmd/         stag-serve · stag-proxy · harness-serve · kbserve · harness
frontend/      one console, talking to both backends
examples/      k8s (real cluster) · pii-demo · zt-ops · scratch — policies live WITH their example
config/        event map + model config
data/          runtime state, gitignored — config DB, the recipe store, tokens, audit logs
tools/         build · up · down · check · hygiene · demo
docs/          recipe authoring · the gating proxy
planning/      every design decision, including the ones we got wrong
```

## Development

```bash
tools/find.sh escalation approval   # find the code that does a thing (keyword index, not grep)
tools/check.sh                      # gofmt · vet · test · ARCHITECTURE · typecheck · index · hygiene
tools/sbom.sh                       # SBOM of the shipped images + a copyleft gate
```

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/development.md](docs/development.md).

`tools/hygiene.sh` exists because both bugs it catches actually happened here: a `.gitignore` pattern
that silently swallowed 19 source files (the build kept working — only a fresh clone was broken), and a
secrets file that stayed tracked because `.gitignore` does nothing to an *already-committed* file.
Neither announces itself. Run it in CI.

## Docs

- [SECURITY.md](SECURITY.md) — the threat model. **Read the non-goals as carefully as the guarantees.**
- [docs/recipe-authoring.md](docs/recipe-authoring.md) — the policy language.
- [docs/mcp-gating-proxy.md](docs/mcp-gating-proxy.md) — how the gate speaks MCP.
- [docs/docker.md](docs/docker.md) — containers, and why the secrets are split across them.
- [docs/development.md](docs/development.md) — the dev tools and the one architectural rule.
- [CONTRIBUTING.md](CONTRIBUTING.md) — how to work on this (and the one rule that is not negotiable).
- [INDEX.md](INDEX.md) — a generated map of every component and what it is for.
- [planning/](planning/) — the full design record.
