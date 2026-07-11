# 04 — Adapter Surface

This is the developer-facing product: how someone defines a recipe and brings their own agent. It
is the "open at the edges" half of the governing law. The kernel (binder, gate, declassifier) is
closed; everything in this doc is the typed boundary a developer works against.

## Recipes are playbook-shaped

A recipe is declarative YAML, in the spirit of an Ansible playbook: a list of steps, each naming an
adapter and its arguments, with a separate registry for the things the developer brings (their
model, their tools). That shape gives the policy-as-data payoff for free: the recipe is
inspectable, diffable, and signable, so an assessor can read what an agent is allowed to do without
reading Go.

Strawman recipe (an agent that triages an alert and proposes a remediation):

```yaml
recipe: triage-and-remediate
version: 1

ingredients:                      # inputs, each declares where it came from
  alert:
    from: $input
    trust: untrusted              # arrived on the bus, unverified
  runbook:
    from: retriever.runbooks
    trust: untrusted:rag          # retrieved: informs, cannot instruct
  host_facts:
    from: pip.inventory
    trust: authoritative          # signed facts

steps:
  - id: classify
    kind: propose                 # the developer's agent lives here
    adapter: model.triage
    inputs: [alert, runbook, host_facts]
    output: { schema: remediation_proposal }   # constrained, enumerated actions

  - id: decide
    kind: gate                    # deterministic, not the model
    policy: policies/remediation.rego
    over: classify.output

  - id: act
    kind: sink
    when: decide == allow
    adapter: tool.remediate
    sensitivity: authoritative
    args:
      action: "{{ classify.output.action }}"   # trust-checked render
      host:   "{{ host_facts.id }}"

release:                          # the declassifier, made visible
  - field: act.args.action
    from_class: untrusted
    allow_if: in_set(actions.approved)          # closed, enumerable set
    record: true                                # emits a ReleaseEvent

egress:
  record: signed
  connectors:
    - webhook: https://prooflayer.internal/api/scans
```

The `act` step is the teaching moment. `classify.output` is derived from `untrusted` ingredients,
so its label is `untrusted` by propagation. It cannot reach the `authoritative` sink unless it
passes the `release` rule (the action is in a closed approved set). That crossing is the
declassifier doing its job, and it is right there in the recipe for a human to audit.

## Three deliberate departures from Ansible

Ansible is the right shape and a cautionary tale, both.

1. **Trust is a first-class field.** Every ingredient declares an origin and a trust class. Ansible
   has no concept of this; it is the novel half of the schema and the reason the recipe exists.
2. **No imperative creep.** Ansible started declarative and drifted into a bad programming language
   in YAML (`when:`, `register:`, `loop:`, nested templating). That drift destroys the property the
   recipe is built to have: that a different component than the executor can reason about it before
   it runs. Control flow stays in a small fixed set of node kinds (`propose`, `gate`, `sink`,
   `branch`, `foreach`, `exit`), never in free expressions. The `when: decide == allow` above is a
   guarded transition over a fixed verdict, not a general condition.
3. **Templating is trust-aware.** `{{ ingredient }}` is not string substitution; it is a render the
   gate checks. Rendering an untrusted value into an authoritative sink is the exact thing that gets
   refused, or routed through `release`. Ansible renders blindly; that blind substitution is the
   confused-deputy hole StAG closes.

## The node kinds

A deliberately small, closed set. Adding a kind is a kernel change, not a recipe author's option.

| Kind | What it does |
| --- | --- |
| `propose` | the developer's agent runs here, taking labeled ingredients, emitting a constrained proposal |
| `gate` | the deterministic PDP decides allow / deny / escalate over a proposal |
| `sink` | an authoritative or benign action, gated, rendered trust-aware, recorded |
| `branch` | a guarded transition over a fixed verdict or enum |
| `foreach` | bounded iteration over a runtime list; the label is handed off at the boundary |
| `exit` | terminate the recipe with an outcome |

## The adapter contract

Adapters are how the developer brings their agent. Each declares a trust posture; that declaration
is the entire developer-facing API surface.

- **Model adapter** (`model.*`). Fixed as an untrusted-until-gated proposer. It takes labeled
  ingredients and emits a constrained proposal. It never authorizes anything.
- **Tool / actuator adapter** (`tool.*`). Declares its **sink sensitivity** (`authoritative` or
  `benign`). Authoritative sinks are gated and can require a release for tainted arguments; benign
  sinks are recorded but not release-gated.
- **Retriever adapter** (`retriever.*`). Declares its **output trust class** (typically
  `untrusted:rag`). Its output informs a proposal but can never become an instruction.
- **PIP adapter** (`pip.*`). A source of signed, `authoritative` facts. This is the only source
  from which trusted instructions may be drawn.

Registration is config-time and local, separate from the recipe (the inventory side of the
playbook). The recipe references `model.triage`; the registry says what endpoint and credentials
`model.triage` resolves to. The recipe stays portable; the bindings are deployment-local.

## Authoring workflow

- **Author in YAML.** Human-facing, playbook-like.
- **`stag lint recipe.yaml`.** Author-time check, extending Ratchet's flow linter for trust classes
  and sinks. This is where "an untrusted ingredient reaches an authoritative field with no release
  rule" becomes a lint error, not a runtime surprise. The linter is load-bearing: in a broker
  product a recipe that silently drops a label fails open, so the author-time guarantee that every
  untrusted-to-authoritative crossing has a declared release is the actual security property, not a
  nicety.
- **Register adapters.** Point `model.triage` at an endpoint; declare `tool.remediate` as an
  authoritative sink.
- **`stag run`**, or the broker serves the recipe and the agent calls in.
- **Canonicalize and sign.** YAML in for people; canonical JSON is the signed artifact and the
  hashed record.

## Integration tiers: bringing an existing agent

How someone who already has, for example, a LangChain agent links in. Easiest to strongest. The
enforcement call is always synchronous; the webhook is only egress.

### Tier 1: MCP gateway (least code, strongest boundary)

If the tools are MCP servers (LangChain speaks MCP via adapters), StAG sits as an MCP proxy between
the agent and the real servers. The agent points its MCP client at StAG's endpoint. Every tool call
flows through StAG, which gates it and forwards or refuses. Zero StAG-specific code in the agent.
MCP is already a boundary protocol, so this is the cleanest single choke point.

### Tier 2: SDK-wrapped tools (native framework)

For native tools that are not MCP, a thin client SDK wraps a tool so invocation routes to the
broker synchronously:

```python
from stag import Broker
broker = Broker("localhost:7777")   # the Go broker

@broker.gated(sink="authoritative", recipe="triage-and-remediate")
@tool
def remediate(action: str, host: str) -> str:
    ...
```

The wrapper ships the call to StAG, blocks on the verdict, and raises on deny so the real function
never runs. The developer declares the sink's sensitivity at the wrap point.

### Tier 3: proxy the model too (full loop)

Route the agent's model calls through StAG as well, so it sees the proposal and binds the labeled
ingredients at the source. More setup, and it is where the full information-flow picture appears
instead of just the final action. This is the rung that turns the drop-in gate into full structural
assurance (see the assurance spectrum in [01-architecture.md](01-architecture.md)).

## Two surfaces, do not conflate them

- **Enforcement is synchronous and inline** (MCP proxy or gRPC/HTTP decision call). The action
  blocks on the verdict. This is where gating happens.
- **Egress is asynchronous** (webhook to ProofLayer, a SIEM, a verifiable log). The decision already
  happened; this is how the signed record leaves.

A webhook cannot gate, because the action would already be done by the time it fired. Say this
plainly in the product docs, because it is the most common mental-model error.

## Authentication is not trust class

An adapter can be perfectly authenticated (mTLS channel, verified identity) and still deliver
`untrusted` content. A fully identity-verified MCP server's output is still untrusted-until-gated.
Two independent axes. The adapter surface must never let "the channel is authenticated" set a trust
class. Identity is consumed from the external authority; trust class is declared per adapter and
propagated by the binder.
