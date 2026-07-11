# 06 — Positioning

**Register note.** This doc is intentionally marketing-heavy. The other docs in this folder are
tight and technical; this one is the messaging house that feeds a landing page and a waitlist. It
still obeys the two hard rules: no dishonest labeling (StAG is in development, the page sells early
access, not a shipped product), and no hype words. Marketing-heavy here means sharp and confident,
not inflated.

---

## Positioning statement

**For** regulated teams deploying autonomous AI agents that take real action, **StAG** is a
deterministic assurance broker that gates every action an agent proposes and leaves a signed record
an assessor will accept. **Unlike** observability tools that watch the agent and guardrails that
filter the prompt, StAG decides, deterministically, whether each action is allowed, and it cannot be
skipped, because the gate that produces the evidence is the same gate that lets the action through.

One line: **structural assurance for AI agents.**

## The category

There is a gap in the tool landscape, and everyone is standing on the wrong side of it.

- **Agent frameworks** (LangChain, LangGraph) give the agent capability. They have no gate and
  produce no evidence. Remove the tracing and the agent still runs.
- **Observability** (LangSmith, OpenTelemetry) gives you traces. A trace is passive, it is a
  removable add-on, and it is not a control. It tells you what the agent did after it did it.
- **Guardrails and gateways** (prompt filters, low-friction proxies) give you probabilistic
  detection. A regulator does not want probabilistic. They want a decision they can reproduce.

StAG plants its flag on the missing side: **the assurance is structural.** The deterministic check
that produces the evidence is the same gate that advances the action. You cannot take the action
without producing the proof, because the proof-producing step is the control flow, not a logger
beside it. Remove it and there is no action.

## The problem (the wedge)

Agents can act now. They open tickets, move money, change access, touch production. Most of them do
it unwatched: by industry estimates roughly half of production agents run unmonitored while the
fleets double every few months. This is not an awareness gap. Teams know. It is an execution gap:
there is no clean way to let an agent act and still be able to defend what it did.

Three things nobody deploying an agent can do today:

- Nobody is gating the action. They are watching it.
- Nobody can reproduce why the agent did what it did, because a non-deterministic model made the
  call.
- Nobody can hand an assessor a record they will accept, because the log is mutable and written
  beside the system, not produced by it.

For a team under a mandate (critical infrastructure, financial operations, an EU AI Act high-risk
system), that is not a rough edge. It is the reason the agent cannot go to production at all.

## What StAG is, in one breath

A single Go binary that sits at your agent's tool boundary. Your agent proposes an action; StAG
returns allow, deny, or escalate from a deterministic policy, not the model; and it emits a signed,
verifiable record of the decision. Bring your own agent, in any framework. You do not rewrite it.
You route its actions through the gate.

## Messaging pillars (the differentiators that survive scrutiny)

Four claims, each defensible to a skeptical engineer.

**1. Reproducible authorization.** The model proposes; a deterministic policy decides. The
authorization is reproducible even though the model is not. That is the audit primitive a regulator
actually wants: not "we are 94 percent confident," but "here is the rule that fired, and it will
fire the same way every time."

**2. Provenance that drives the decision.** StAG tracks where every input came from and refuses to
let untrusted data reach a field that authorizes an action. The gate does not judge whether the
enrichment was "good," an unsolvable and dishonest promise. It answers a deterministic question:
did untrusted-origin data flow into a field the policy reads?

**3. Deterministic downstream of the planner.** Detection tools sit upstream and guess. StAG sits
downstream and bounds. It does not try to detect a bad proposal probabilistically; it makes a bad
proposal unable to authorize anything, because the model is never in the decision path.

**4. Model-agnostic by construction.** The assurance lives in the structure around the model, not in
the model. The next model does not change the gate. You are not buying a defense tuned to this
quarter's jailbreaks; you are buying a boundary that holds regardless of what runs inside it.

## The hard thing we are actually building

The honest center. Every serious system that controls information flow has a declassifier: the
deliberate, audited act of letting a controlled value cross into a field that authorizes action. It
is the part the literature says leaks, and most tools either do not have one or hide it in code
where no one can inspect it. StAG's declassifier is the reason the product exists: a closed, legible
release engine where every crossing is a declared, signed, inspectable rule, not an invisible branch
in someone's script. It does not claim to make a release provably safe. It makes every release
something a human can catch. That honesty is the differentiator, not a caveat to bury.

## Who it is for

The near-term buyer is narrow and vertical on purpose: the regulated, high-blast-radius operator who
needs defensible, reproducible authorization and will tolerate integration because a compliance
mandate forces the buy.

- **Beachhead:** teams under a mandate that makes an unprovable agent a non-starter. Critical
  infrastructure, financial operations, defense and high-assurance federal, EU AI Act high-risk
  systems.
- **The buyer:** the person who owns "this agent cannot go to production until I can prove what it
  does." A CISO, an ISSM, a head of AI risk or platform security.
- **Not the buyer:** a team with no mandate and no blast radius. They will use a gateway and move on.
  That is fine. StAG is for the room where a wrong action is a real incident and a wrong record is a
  finding.

## Why us (proof of seriousness)

Built by people who have sat in the assessment, not read about it. The founding doctrine is one line
carried across a portfolio: verify, do not trust. StAG is the third time we have built the
deterministic-gate shape. Ratchet gates AI code generation. ProofLayer signs and logs compliance
evidence against an RFC 6962 transparency log. ESP verifies endpoint state. StAG points the same
kernel at the agent's action path. The pattern is proven; StAG is the product form of it. (See the
reference architecture that specifies the design in full.)

## Objection handling

- **"Is this another guardrails tool?"** No. Guardrails filter the prompt and hope. StAG decides the
  action, deterministically, and proves it. Different layer, different guarantee.
- **"Do I have to rewrite my agent?"** No. You route its tool calls through the broker. The lightest
  integration is a proxy in front of the tools your agent already calls; the agent code does not
  change.
- **"What about latency?"** The gate is a deterministic policy check on the action path, not on the
  model's reasoning path. The cost lands where an action must be defended, not on every token.
- **"Does it lock me to a model or a framework?"** The opposite. The assurance is in the structure
  around the model, so it is model-agnostic and framework-agnostic by construction.

## Landing-page raw material

**Headline candidates**
- Structural assurance for AI agents.
- Your agent can act. Can it prove it should have?
- Stop watching your agents. Start gating them.
- Bring your agent. Keep the proof.

**Subhead**
> StAG is a broker that sits at your agent's tool boundary, turns every proposed action into a
> deterministic allow, deny, or escalate, and leaves a signed record an assessor will accept. Bring
> your own agent, in any framework. You do not rewrite it.

**Three-up value blocks**
- **Decide, do not watch.** A deterministic policy authorizes every action. The model proposes; it
  never decides.
- **Prove it, do not log it.** Each decision is a signed, verifiable record, produced by the gate,
  not written beside it.
- **Bring your agent.** Any framework, any model. Route the actions through the boundary; leave the
  agent alone.

**Primary call to action**
> Join the early-access waitlist. (Design partners help shape the recipe format and get first
> integration support.)

## Honesty guardrails for the page (non-negotiable)

- **Label the status.** StAG is in development. The page sells a waitlist and a design-partner
  program, not a shipped product. Use "in development," "early access," "coming," never present-tense
  claims that imply general availability.
- **Describe design intent as intent.** It is honest to say "StAG is built to gate every action";
  it is dishonest to imply it is already gating actions in production for customers. When in doubt,
  phrase as what the product does by design, inside the in-development frame.
- **The sibling engines are real; StAG-the-product is not yet.** It is true and strong to say Ratchet
  and ProofLayer exist and prove the pattern. It is false to let a reader think StAG ships today
  because they do.
- **No unprovable numbers.** The "half of agents run unmonitored" style stat needs a real citation on
  the page or it gets cut. No invented percentages.
- **State the ceiling once, plainly.** Somewhere on the page or the FAQ, say what StAG does not do
  (it does not harden the model, it does not manage identity, it does not claim a release is provably
  safe). The buyer who trusts the page is the buyer who reads the limits and finds them honest.
- **No em dashes, no hype words.** House standard. The work is interesting enough without them.
