# StoaGraph

**Verifiable control for AI agents.** An agent proposes; a deterministic gate disposes — with no model
in the decision path. A hijacked, prompt-injected, or simply wrong model can propose anything; it cannot
make the gate release a value your policy rejects.

> The model has a flashlight. The gate has the map.

Open source, Apache-2.0. No held-back edition.

## See it in 60 seconds — no model, no API key

```bash
curl -sSL https://raw.githubusercontent.com/scanset/stoagraph/v0.1.0/install.sh | sh
stoagraph up      # mints your secrets, pulls the signed images, starts, prints a login link
stoagraph demo    # loads a support-agent containment scenario
```

Now watch an agent try — and fail — to leak a customer's SSN:

```
fetch_user_profile(123)                          ALLOW   returns Alice's record, INCLUDING her SSN
send_external_reply("Your SSN is 000-12-3456")   DENY    never reaches the tool
send_external_reply("Hi Alice, you're unlocked") DENY    <- innocent, and STILL blocked
send_external_reply("tmpl:account_unlocked")     ALLOW   an approved template
```

**Look at the third line.** An innocent message is blocked too — because *no free-form value can cross
at all*. The policy never scans for SSNs; it permits four template ids. There is no clever phrasing that
becomes an approved template id, which is exactly why a jailbroken model cannot get around it.
Containment is **structural, not content-based**.

<sub>Piping a script into your shell, from a product that says "don't trust — verify"? Fair. `install.sh`
lives in this repo at the tag it installs, verifies the binary's SHA-256 against cosign-signed
checksums before running anything, and prints `cosign verify` for the images. `curl -sSLO …/install.sh
&& less install.sh && sh install.sh` if you'd rather read first. Have Go?
`go install github.com/scanset/stoagraph/stoa-kernel/cmd/stoagraph@latest`.</sub>

## Make it yours

Everything below the demo. A fresh gate starts **empty** — nothing is permitted until you author it.

- **Log in.** `stoagraph up` printed a one-click link (the keys ride in the URL fragment, so there's no
  token to paste). Reprint it with `stoagraph console`. Open **http://localhost:3000**.
- **Connect a model.** Copy `config/models.example.json` → `config/models.json`, add a key. **The gate
  never sees it** — only the orchestrator does.
- **Add your own tool.** [`examples/custom-tool/`](examples/custom-tool/) — a copy-paste MCP server and a
  12-line recipe. Write one function, gate one argument, and the agent can call your tool on the values
  you allow and *provably* nowhere else. ~5 minutes.
- **Run the real demo** against a live Kubernetes cluster: an incident event → the dispatcher binds a
  session → the agent reads infra facts as untrusted context, investigates with gated reads, proposes a
  fix to **prod** → the gate **escalates** and waits for a human. See [`examples/k8s/`](examples/k8s/).

## How it works

**Two pieces, and the split is the product:**

| | | |
|---|---|---|
| **`stag`** | the **gate** | Deterministic kernel, MCP proxy, policy, audit, approvals. **No model, no API keys.** |
| **`harness`** | the **orchestrator** | Dispatcher, agent loop, model connections. **Holds the keys.** |

That separation is *enforced*, not intended: [`architecture_test.go`](stoa-kernel/architecture_test.go)
fails the build if any gate package — or either gate binary — imports orchestrator code. The gate can be
trusted with your infrastructure precisely because it is *provably incapable* of reaching your keys.

**The agent's only wire to the world is the gate.** It has no sandbox, no direct tool access, no
network, no credentials — it reasons and proposes, and the single channel by which a proposal becomes an
action is StoaGraph. Both directions cross it:

| Channel | MCP surface | What the gate does |
|---|---|---|
| **ACT** | `tools/call` | **allow / deny / escalate** — forward-iff-cleared; a denied call never reaches the tool. |
| **READ** | `resources/read` | **label + record** — context is stamped untrusted at origin, unbypassably, and audited. |

This is *complete mediation*: a jailbreak changes what the model asks for, not what it can reach — and
what it can reach is exactly what the gate hands it.

**Humans stay in the loop, and the machine can't forge it.** When policy says an action needs a person,
the gate escalates and holds the call; a human approves, minting an ed25519 signed release bound to that
exact action. The orchestrator can *poll* for the decision but **cannot approve** — that's a separate
secret it is never given. See [SECURITY.md](SECURITY.md).

## The trust invariant (the part most systems get wrong)

The untrusted stamp on context is **positional, not taint-tracking.** An LLM launders taint — text goes
in, a tool call comes out, and there's no reliable way to know which output bytes came from which input.
Anything claiming to propagate a taint label *through* a model is lying. StoaGraph doesn't try:

- **Into the model** — the label's only job is *placement*: untrusted context goes in the input slot,
  never the instruction slot, so it can't rewrite the agent's goal.
- **Out of the model** — there is **no carried label.** Every proposal is *presumed untrusted*; the gate
  re-derives trust **at the sink**, from the policy rule.

The only promotions from untrusted → cleared are a rule firing (`set_membership`, `numeric_range`,
`signed_equality`), each emitting a recorded release event. Poisoned context can change *what the agent
proposes*; it cannot make the gate *release* a value the rule rejects. A bad read wastes a turn; it does
not breach.

## Layout

```
stoa-kernel/   one Go module — the whole backend
  stag/        the GATE         (kernel, policy, proxy, auth, audit, approvals)
  harness/     the ORCHESTRATOR (dispatch, agent loop, models)
  cmd/         stag-serve · stag-proxy · harness-serve · kbserve · …
frontend/      one console, talking to both backends
examples/      custom-tool (start here) · k8s · pii-demo · zt-ops
config/        event map + model config        data/  runtime state (gitignored)
tools/         build · up · down · check · hygiene · demo · sbom · find
docs/ · planning/
```

## Development

```bash
tools/find.sh escalation approval   # find the code that does a thing (keyword index, not grep)
tools/check.sh                      # gofmt · vet · test · ARCHITECTURE · typecheck · index · hygiene
tools/sbom.sh                       # SBOM of the shipped images + a copyleft gate
```

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/development.md](docs/development.md).

## Docs

- [SECURITY.md](SECURITY.md) — the threat model. **Read the non-goals as carefully as the guarantees.**
- [docs/recipe-authoring.md](docs/recipe-authoring.md) — the policy language.
- [docs/routes.md](docs/routes.md) — binding a tool to a policy: why an unrouted tool is denied, and which arguments a route must gate.
- [docs/mcp-gating-proxy.md](docs/mcp-gating-proxy.md) — how the gate speaks MCP.
- [docs/docker.md](docs/docker.md) — containers, and why the secrets are split across them.
- [examples/custom-tool/](examples/custom-tool/) — bring your own tool in ~5 minutes.
