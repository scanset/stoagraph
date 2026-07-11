# 01 — Architecture

## The pipeline

```
Untrusted world ─► Context Binder ─► Agent (proposer) ─► Gate/PDP ─► Declassifier ─► PEP ─► actuator
                   origin-tagged      non-deterministic   deterministic  release       enforce   (or human)
                   scoped inputs      plan/proposal       allow/deny/    (if crossing)  + record
                                      (external, BYO)     escalate
```

Four mechanisms, each governing a different thing:

1. **Context binding** controls *what enters each step and from which declared origin*. This is
   the information-flow (IFC) layer: it computes the trust label.
2. **The gate (PDP)** is the deterministic decision downstream of the planner. It decides allow /
   deny / escalate over the proposal. This is the attribute-based (ABAC) layer: it consumes the
   label as one attribute among others.
3. **The declassifier** is the only path by which an untrusted-labeled value may reach an
   authoritative field. It is the hard center; it has its own doc
   ([02-declassifier.md](02-declassifier.md)).
4. **The record** is what happened, for audit and rollback. Descriptive, not normative, but
   produced by the same gate that enforces, so it cannot be skipped.

Organizing principle: **authenticate the source, distrust the payload, authorize by origin at the
sink.** Trust is conferred by verified origin and the gate's verdict, never by presence in the
prompt.

## IFC computes, ABAC decides (keep them separate)

A scalar "trust score" is a lossy collapse. The design keeps origin, schema-validation,
endorsement, and sink-sensitivity as separate attributes a rule can read.

- **The binder is the IFC layer.** It computes the taint label by propagating it across the bind
  graph. A field that drives the gate inherits the taint of everything in its lineage.
- **The gate is the ABAC layer.** It consumes the label as one attribute among blast-radius,
  change-window, protected-prefix, and the rest.

Taint is not the protected resource. The **sinks** (authoritative, gate-driving fields) are the
protected resource. Taint is one attribute *in* the decision that protects them. This matters
because it means taint is not a second gate bolted on; it is a new input to the one gate that
already exists.

## Dynamic taint, not a precomputed graph

Propagation rides the value as the engine runs: read the class next to the value in state, join on
a bind, stamp `untrusted` on a retrieval source. Same control flow, one extra field. It is not
ahead-of-time path enumeration, which is both harder and unsound here, because a retrieval source
can pick its library from a runtime slot and a `foreach` iterates a runtime-sized list. The
graph's edges are partly chosen at execution time.

A lint-time "can this recipe possibly leak?" static check is a real, separate, later feature that
declarative recipes (policy-as-data) make checkable. See
[04-adapter-surface.md](04-adapter-surface.md).

## The broker shape

StAG is a broker at the tool boundary, not an in-process library. The consequences:

- **Enforcement is synchronous and inline.** The agent's action blocks on StAG's verdict before
  anything touches the world. This is a request/response call (an MCP proxy or a gRPC/HTTP decision
  endpoint), never a webhook. A webhook is fire-and-forget; you cannot gate with one.
- **Egress is asynchronous.** The signed record leaves for a verifiable log, a SIEM, or ProofLayer
  after the decision. That is where webhooks live.
- **The process boundary is the closed gate.** In-process IFC in a dynamic language is where the
  label gets dropped by an untyped assignment. A process boundary makes the label impossible to
  drop between the agent and the sink, which is exactly where the reference architecture says trust
  stops being conferred by presence.

## The assurance spectrum

How much of the guarantee a customer gets depends on how much of the data path StAG sees. This is
a spectrum, not a switch.

- **Drop-in gate (front the tools).** StAG sees the tool call, not the reasoning. It gives the
  deterministic allow / deny / escalate over the proposal (ABAC) and the signed record. It cannot
  trace the lineage of arguments it never saw formed, so taint is partial.
- **Recipe-owned ingredients (StAG serves context and tools).** StAG serves the retrieval and
  provides the labeled inputs, so it computes full taint, runs the declassifier, and records the
  ReleaseEvent provenance. Full information-flow control.

The product's job is to make each rung a small, obvious step, so "bring your agent unchanged"
lands at the drop-in gate and climbing to full structural assurance is integration, not a rewrite.

## Two postures, one kernel

A recipe can either own the loop or gate an external one. Same kernel, different amount of control.

- **Gate an external loop (broker, default).** The agent drives in its own framework; StAG gates
  the tool and actuator calls. Maximum "bring your own agent." Consistent with the reference
  architecture (propose, then PDP, then PEP at the sink). The recipe is the assurance contract on
  the action path.
- **Own the loop (strict mode, optional).** The agent is slotted into the propose step and StAG
  controls sequencing. Strongest assurance, for a buyer who wants StAG to own the order of
  operations too. Same binder, gate, declassifier, and record.

## Go implications

Go is right for the kernel (static binary, no runtime, air-gappable, and Ratchet already proves
the shape). Two implications to hold deliberately:

- **Go picks the integration model.** A Go binary cannot be a Python in-process library, so "bring
  your own agent" means a process boundary. That is the more enforceable shape, not a compromise.
- **Canonical hashing is a discipline, not a default.** Go has no `BTreeMap` and map iteration is
  randomized. ESP's determinism (every hash-feeding map is a `BTreeMap`, every hash-feeding vec is
  sorted) has to be rebuilt in Go by explicit key-sorting and fixed field order before hashing. The
  clean split: authors write YAML, StAG canonicalizes to JSON, and only the canonical JSON is ever
  hashed. Humans never touch the bytes that get signed.

## What lives where (verified against the sibling engines, 2026-06)

The engine shape StAG re-implements is already proven in Ratchet's Go code, and the taint gap is
small and real:

- The binder knows the bind source (`from` / `ref` / `search`) at bind time, then discards it into
  a bare string map (`go_src/internal/chain/engine.go:461-482`, discard at line 479). A parallel
  `stateClass` map, populated at that same point, is where the label is computed.
- The action sink renders slot values straight into tool arguments with no class check
  (`engine.go:606`). One refusal check there is the gate at the sink.
- The `foreach`/`Run` boundary drops any label at the call site (`engine.go:449` into the child's
  `state["$input"] = input` at line 125). The label hand-off must live at the `Run` signature.

None of this is a rewrite. It is a parallel label map, a join on bind, one refusal check, and a
signature change. The attestation half is carried from ESP's patterns and the verifiable log is
consumed from ProofLayer or Rekor; both are covered in
[03-record-and-attestation.md](03-record-and-attestation.md).
