# 02 — The Declassifier

This is the hard center. Everything else in StAG is buildable or already built next door. The
declassifier is the one part the literature says leaks, and building a legible, kernel-closed one
is the reason StAG exists as a product rather than a wrapper.

## Why it is unavoidable

Pure information-flow control is useless on its own. Honest provenance means untrusted data touches
almost everything, so either the gate escalates the entire world to a human (autonomy dies) or you
write rules that deliberately clear the taint on specific values. That deliberate clearing is
declassification. Every real IFC system has one, and the whole security of the system collapses
onto how it decides. The hard problem is not tainting. It is **overtainting**, and the declassifier
is the only relief.

## What it is, concretely

The declassifier is the only path by which an `untrusted`-labeled value may reach a
`trusted`/authoritative field (a field the gate reads, or an argument to an authoritative sink). It
is a closed, policy-as-data release engine. It takes a tainted value and a declared rule, and it
either releases the value (clearing the taint for that specific crossing) or refuses.

It is not a grader of content. It never asks whether the value is "good." It asks whether this
specific release is permitted by a rule authored in advance, and it records the crossing either
way.

## The four dimensions (each maps to an attack)

Sabelfeld and Sands decompose declassification into four dimensions. Each is an attack surface.

- **What** is released (the content). Attack: smuggle authoritative content into the released part.
- **Who** may release (the authority). Attack: get a lower-privileged component to declassify.
- **Where** release happens (the locus). Attack: move the release point so taint clears too early.
- **When** release happens (time and ordering). Attack: replay a value that was cleared under old
  conditions.

A declassifier design has to answer all four, and the recipe has to make all four inspectable.

## The core difficulty: laundering

Declassification deliberately creates an information channel, and any channel an attacker can
influence is exploitable. This is laundering:

- Schema or format match? Craft a value that matches the schema and still carries the payload.
- Clean summarizer? Craft content whose summary carries the payload.

"Robust declassification" demands that the released thing be invariant under an adversarial choice
of the tainted data. That is brutally hard for anything beyond a fixed-format field.

## The five strengths (weakest to strongest)

1. **Schema / format validation.** Weak. Structure is not safety; a well-formed value can still
   carry an attack.
2. **Provenance narrowing.** Strong *if* the origins are truly separate. Release only from a source
   the attacker cannot reach.
3. **Trusted transform.** Only as strong as the transform's invariance under adversarial input.
4. **Human endorsement.** Strong in authority, weak in practice (rubber-stamp) and it does not
   scale.
5. **Quantitative / capability bounds.** The CaMeL move: release only from a small, closed,
   enumerable set fixed in advance.

## The unifying rule

**Declassification is safe only to the degree the released thing is drawn from a small, closed,
enumerable set defined in advance.** The moment the declassifier must judge open-ended content, you
are back at the unsolvable problem. This is the single design constraint that governs the whole
component:

- A release rule that says "the action is one of {restart, isolate, notify}" is safe, because the
  output space is closed and enumerable.
- A release rule that says "the summary looks clean" is not a release rule; it is a content grader
  wearing one, and it fails to laundering.

The recipe language must make the first easy and the second hard. Release predicates are
deliberately narrower than a general policy language: membership in a declared set, equality
against a signed value, a bounded numeric range. No free computation, no content inspection, no
Turing-completeness.

## What the language buys: legibility, not soundness

Be honest about the ceiling. A policy-as-data declassifier does not make a release provably safe.
It makes the release a **declared, signed, diffable claim** that a human can inspect before it runs
and audit after. That is legibility, and legibility is all the language buys. It is still worth
building, because the alternative (`if schema_valid { declassify() }` buried in code) is a decision
the executor makes about itself, invisible except as effect, that no human ever gets to catch.

## Why policy-as-data, precisely

Not mainly "separates intent from execution" (a convenience) and not mainly "no arbitrary
execution" (that protects against a malicious policy, but the declassifier is yours). The
load-bearing reason:

**Policy-as-data makes the release rule an inspectable object that a different component than the
executor can reason about before it runs.** It separates *the act of declassifying* from *the
authority to declassify*, and it makes the rule a signed, attributable, diffable artifact rather
than an invisible runtime branch. That separation is the only thing that ever lets a human catch
that a rule was too weak.

ESP already proved you can compile that shape with the limits baked into the binary. StAG carries
the pattern, narrowed to release predicates.

## The kernel line

The declassifier is kernel. It is never author-configurable in the sense of "write your own release
logic in a general language." The author declares *which closed set* a value may be released
against and *who* may authorize it; the author does not get to author the release mechanism. A
freely author-configured declassifier is a labeled gun with the safety sold separately. Open at the
edges (the sets and authorities are declared per recipe); closed at the gate (the release engine
itself is fixed StAG code).

## The record of a crossing: the ReleaseEvent

Every release emits a first-class, hashed `ReleaseEvent`:

> input X of class `untrusted`, from origin O, was released to authoritative field Y under rule R,
> authorized by actor A, at ordering position N.

This is the object ESP never had to represent. ESP compares a value to an expectation and discards
it, so nothing crosses a boundary and there is nothing to attest. StAG's gate must attest the
crossing ESP avoids. The ReleaseEvent is where the four dimensions (what, who, where, when) become
recorded fields, and it rolls into the signed leaf like any other criterion. See
[03-record-and-attestation.md](03-record-and-attestation.md).

## The honest ceiling, stated for the doc set

- Declassification correctness is expressible and auditable through policy-as-data, never provably
  correct. It is the unsolved center for everyone in this space, not just StAG.
- The safe cases are the closed-set cases. StAG's job is to make the safe cases ergonomic and the
  unsafe cases visibly unsafe, not to claim it solved the general problem.
- A propagation bug in the binder (over- or under-taint) is a silent authorization failure that a
  perfect declassifier never catches, because the declassifier only sees the label it was handed.
  The thing to trust shifts from the gate to the IFC feeding it. See
  [05-open-problems.md](05-open-problems.md).
