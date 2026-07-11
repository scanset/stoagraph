# 05 — Open Problems

The honest list, in priority order. These are not reasons not to build. They are the things the
product must state plainly rather than paper over, because the buyer for structural assurance is
exactly the buyer who will read the spec and find the gap you hid.

## 1. Declassification correctness

Expressible and auditable through policy-as-data, never provably correct. This is the unsolved
center for everyone in this space, not just StAG. The mitigation is the closed-set rule (release
only from a small, enumerable set fixed in advance) and legibility (every release is a declared,
signed, diffable claim a human can catch). What StAG does not claim: that a release is provably
safe. See [02-declassifier.md](02-declassifier.md).

## 2. Overtainting kills autonomy

Honest provenance taints almost everything, so a naive gate escalates the entire world to a human
and autonomy dies. Declassification rules are the only relief, and they leak. The product tension is
permanent: too little declassification and nothing runs; too much and the gate is decorative. The
recipe linter and the ReleaseEvent record are how a team tunes this with their eyes open, not how
StAG solves it.

## 3. In-policy-but-wrong (RAG steering)

The gate reads the proposal, not the influence. Sink-protection stops untrusted data from reaching
authoritative fields, but it does not stop untrusted enrichment from steering the agent toward an
in-policy-but-bad choice among allowed actions. If every action in the approved set is individually
allowed, the gate cannot catch that enrichment nudged the agent to the worst allowed one.
Counterfactual ablation (re-run without the untrusted slot) detects that enrichment *mattered*, but
it is a monitor and an escalation signal, not a gate, and it doubles inference cost. State this as a
known limit of the sink-protection model.

## 4. The computed label is a new trust dependency

Because the gate now rests on a computed label, a propagation bug in the binder (over- or
under-taint) is a silent authorization failure that a perfect gate never catches, because the gate
only sees the label it was handed. The thing to trust shifts from the gate to the IFC feeding it.
Mitigation: the binder's propagation is small, deterministic, and testable (a parallel label map, a
join on bind, one refusal check), and the recipe linter can prove reachability properties over the
declared graph. But the dependency is real and must be owned.

## 5. Blast-radius estimate is a soft input to a hard gate

Some gate inputs (how destructive is this action, how large is the blast radius) are estimates, and
an estimate feeding a deterministic gate must fail safe: uncertain resolves to escalate, never to
allow. This is a design rule, not a solved problem, and a miscalibrated estimator either escalates
everything (autonomy dies) or waves through something it should not.

## 6. The human gate degrades to a rubber stamp

Escalate-to-human is the honest fallback for what policy cannot express, but under load reviewers
rubber-stamp, and the determinism guarantee's safety net collapses into non-deterministic
authorization wearing a deterministic mask. Mitigations: the "recommend" surface must present
defeaters (why this might be wrong), not a one-tap green approval; approvals must be rate-limited and
paced against fatigue; and each approval is itself recorded as a signed event. None of this makes a
tired human a good gate; they reduce how often the human is the gate.

## 7. Authentication is not trust class

A perfectly authenticated channel can still deliver untrusted content. The two axes (identity of the
producer, trust class of the payload) must stay separate everywhere in the product, or "the channel
is authenticated" silently smuggles in "the content is trusted." This is a discipline the adapter
surface enforces, not a problem that gets solved once.

## 8. Enforcement completeness (broker-specific)

StAG can only gate the sinks that route through it. A sink the agent can reach directly, outside the
boundary, is ungated, and an ungated sink means the label fails open. The product must make it easy
to route every sink through the one boundary (the MCP gateway is best for this, being a single choke
point) and must be able to detect and refuse an un-registered sink. "Open at the edges, closed at
the gate" is only true when every edge actually goes through the gate.

## 9. Taint completeness across the boundary (broker-specific)

In the drop-in tier, StAG sees the final tool call but not the reasoning that formed its arguments,
so it can gate the action (ABAC) but cannot trace the lineage (full IFC). Full taint requires StAG
to be in the data path (serve the retrieval, provide the labeled ingredients). This is the assurance
spectrum from [01-architecture.md](01-architecture.md), restated as a limit: the drop-in gate is
real and useful, but it is not the full information-flow guarantee, and the product must not let a
buyer believe it is.

## What is not on this list

Notably absent, on purpose: identity management, model hardening, output-channel data-loss
prevention, and content trustworthiness grading. Those are non-goals
([00-concept.md](00-concept.md)), not open problems. StAG composes with the systems that own them.
Keeping them off this list is how the product stays tight.
