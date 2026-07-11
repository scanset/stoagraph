# 00 — Concept

## What StAG is

StAG is a deterministic assurance broker for autonomous AI agents. It sits at the boundary where
an agent acts on the world (a tool call, an actuator, an API), and it does four things there:

1. **Binds context by origin.** Every input an action rests on carries a trust label derived from
   where it came from, not from its position in a prompt.
2. **Gates the proposed action.** A deterministic policy, not the model, returns allow, deny, or
   escalate over the agent's proposal.
3. **Declassifies deliberately.** When an untrusted value must reach an authoritative field, the
   crossing goes through a closed, legible release policy, never a silent assignment.
4. **Records the decision.** The whole action becomes a signed, verifiable leaf, structured so a
   party that trusts none of the actors can reconstruct and check it.

The one-sentence position: **structural assurance for event-driven agentic AI, layered on a Zero
Trust architecture.** Differentiator first (structural assurance), domain second (event-driven
agentic AI), frame third (Zero Trust).

## Why "structural" and not "observed"

The deterministic check that produces the evidence is the same gate that advances the action. You
cannot take the action without producing the proof, because the proof-producing step is the
control flow, not a logger beside it. Remove the gate and there is no chain. Other tools
instrument an agent for observability after the fact; StAG's assurance is load-bearing by
construction. That is the whole differentiator, and it is why the record is a byproduct of
enforcement rather than a separate pipeline.

## The extracted-kernel framing

StAG is not "Ratchet plus a trust label." Ratchet is a harness: it drives its own constrained
proposer through a flow it controls. StAG wraps *someone else's* agent loop. It is the assurance
kernel (binder, gate, declassifier, record) lifted out of the harness and pointed outward, so a
customer keeps their own agent framework and routes its actions through StAG's boundary.

This is the cash value of a claim made in the reference architecture: the assurance kernel is
domain-agnostic and extractable. StAG is that extraction, packaged as a product.

## The core thesis

An AI agent is an untrusted, non-deterministic proposer. Safety does not come from making the
model trustworthy. It comes from bracketing it:

- **Upstream:** control what enters the model's context, and tag each fragment with the trust
  level of its source. The context window is an implicit trust zone (everything in it is read with
  uniform authority); origin-bound binding is the micro-segmentation response.
- **Downstream:** place a deterministic gate after the planner. The model proposes; the gate
  decides. The gate's authorization is reproducible even though the planner is not.

Reliability stops scaling with model IQ and starts scaling with the structure around the model.

## The product ambition

Something a developer can adopt and bring their own agent to, in the spirit of a framework like
LangChain, but as a broker rather than an in-process library. Built in Go: a single static
binary, no runtime, self-hostable, air-gappable. Thin client SDKs in the languages agents are
written in (Python first). The agent talks to StAG across a process boundary, which is both the
adoption surface and, not by accident, the enforcement surface (see
[04-adapter-surface.md](04-adapter-surface.md)).

## The governing law

**Open at the edges, closed at the gate.** The thing that makes an integration layer adoptable
(unopinionated, everything plugs in) is the thing that makes information-flow control
unenforceable through it. One unlabeled call severs lineage, and a severed label fails open. The
resolution is to keep the kernel closed and the edges typed:

- **Closed kernel:** taint propagation, the gate, and the declassifier are not pluggable and not
  author-configurable.
- **Typed edges:** models, tools, and retrievers plug in as trust-classed adapters. A tool
  declares its sink sensitivity; a retriever declares its output class; a model is fixed as an
  untrusted-until-gated proposer.

You bring your agent. You do not bring your own gate.

## Scope

StAG secures the runtime action path of an autonomous agent: the moment a proposed action meets a
sink that can affect the world. It is strongest exactly there, and it is deliberately bounded.

## Non-goals (the boundary that keeps it tight)

Stating what StAG is not is part of the design. Each of these is a real system StAG composes with,
not one it absorbs.

- **Not IAM.** StAG does not issue, rotate, or manage identity. It *consumes* cryptographic
  identity from an external authority (SPIFFE/SPIRE or equivalent). Trust class and authentication
  are two independent axes: a perfectly authenticated channel can still deliver untrusted content,
  and StAG never lets "the channel is authenticated" smuggle in "the content is trusted." Identity
  answers who acted; StAG answers whether the action was allowed and provable.
- **Not model hardening.** Nothing in StAG makes a model harder to fool. It makes a fooled model
  unable to act without a deterministic decision and an indelible record.
- **Not output-channel data-loss prevention.** StAG contains exfiltration through the *action*
  path (scoped sinks, blast-radius separation). Exfiltration through the model's *output* channel
  (secrets encoded into a response) is a complementary layer.
- **Not a content-trustworthiness grader.** StAG never asks "is this enrichment good?" That is the
  unsolvable content problem, and a green "looks fine" check manufactures false confidence.
  Instead it asks "did untrusted-origin data flow into a field the policy reads?" A flow judgment,
  deterministic and binary.
- **Not the verifiable log itself.** The append-only transparency log is a consumed component
  (RFC 6962 / Rekor, or ProofLayer's own log). StAG produces the leaf and hands it off. See
  [03-record-and-attestation.md](03-record-and-attestation.md).
- **Not a compliance-control registry.** StAG emits verdicts that a downstream system can map to
  controls (EU AI Act, ISO 42001, NIST). The mapping and the framework content live in that
  system, reached over the egress connector.

## Relationship to the reference architecture

The reference architecture (`/home/local/Ratchet/docs/preview/ZT-Reference.md`) describes a
vendor-neutral pattern: signed action-events, a deterministic PDP, a PEP that guarantees
provenance, a model kept out of the trust path, and a verifiable log. StAG is the implementable
core of that pattern for a single deployment. Where the reference doc says "the model boundary,"
"the PDP," "the PEP," StAG is the running code. Where the reference doc stubs the verifiable log
and the identity authority as consumed, mature components, StAG consumes them over a connector.
