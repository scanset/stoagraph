# 13 — Event ingress: the listener seam (and why it lives outside the gate)

Recorded 2026-07-03. The runtime (Planning/12, DEVLOG U13–U18) runs end to end: it decides and
enforces. What it does not yet have is a production **event source** — the thing that notices an
incident and hands it in. The demo drives events from `scenarios/*.txt` via `stag-incident run`; that
is a test driver, not ingress. This doc fixes where a real listener belongs, why it earns no trust,
and records the two concrete listeners we will build later. **Decision: explain and defer; build after
the egress/actuator work. Message-queue and webhook receivers are both cached as future features.**

## The trigger seam already exists

The runtime exposes one decision method over the stdio JSON-RPC transport (U17):

```
decide { event: "<incident text>" }  ->  { verdict, value, cleared, recommend, denied, events, fired }
```

`serve` reads these requests one-per-line from stdin. So the socket is built; a **listener** is
whatever writes requests into it (out-of-process) or calls `Engine.Decide` directly (in-process). The
listener is not a missing piece of the core — it is glue in front of the core, the same status as an
actuator behind it.

## Where the listener sits: async in, synchronous gate, async out

```
  event source            INGRESS (untrusted)        THE GATE (synchronous)         EGRESS (async)
  ────────────            ───────────────────        ──────────────────────         ──────────────
  Alertmanager  ┐                                    Engine.Decide:
  PagerDuty     ├─POST──▶ listener ──decide req──▶     retrieve → assemble →   ──▶  release events →
  NATS / SQS    ┘         (adapter)                    gate → fire-iff-cleared       audit log (best
  a file drop                                                                         effort, off-path)
                          ▲ owns the event source     ▲ the ONLY authority           ▲ never gates
                            earns no trust               the model can't reach
```

Three placement rules, each load-bearing:

1. **The event is untrusted Input, whatever carries it.** `bind.Assemble` already wraps the event as
   labeled data in `Input`, never `System` (the trust-position invariant, U15). A webhook payload, a
   queue message, and a text file are identical to the gate: untrusted content. The injection we
   proved survivable (U18) *is* a hostile event; the delivery channel changes nothing.

2. **Ingress may be async; enforcement may not (invariant 9 — "a webhook cannot gate").** The listener
   is allowed to receive and enqueue asynchronously. What is forbidden is (a) the listener deciding to
   fire an actuator, or (b) an async callback being the thing that "clears" an action. The
   decide→fire-or-refuse step stays synchronous and in-path inside `Engine.Decide`. The listener hands
   an event in and gets a verdict back; it never enforces.

3. **The listener earns no trust and holds no policy.** It parses its transport, extracts event text,
   and calls decide. It does not know the label vocabulary, the tiers, or the recipe. If a listener
   started making allow/deny choices, the gate would no longer be the single mediator (invariant 10).

Corollary: a compromised or buggy listener can, at worst, deliver a **bad event** — which is exactly
the untrusted-input case the gate is built to bound. It cannot cause an unauthorized effect, because
it is on the left of the synchronous gate, not inside it.

## In-process vs. out-of-process

Two wirings, same seam:

| | Listener → `Engine.Decide` (in-process) | Listener → `stagd` stdin (out-of-process) |
| --- | --- | --- |
| Coupling | one binary; the listener imports `runner` | two processes; JSON-RPC over the U17 transport |
| Isolation | shared address space | the gate runs as its own process (crash/faults isolated) |
| When | the default; simplest, fewest moving parts | when the host wants process isolation or a language boundary |

v1 of any listener should be **in-process** (call `Engine.Decide`). The stdio boundary is already
there for the day process isolation is wanted; it does not need to be the first wiring.

## The two listeners we will build (deferred)

Both are adapters in `cmd/` (or a small `ingress` package of pure adapters), never in the kernel/broker/runner core.

### A. Message-queue consumer (leaning primary)

Subscribe to an incidents topic; decide per message; publish the decision.

```
subscribe incidents.*
  msg ─▶ Engine.Decide(msg.event) ─▶ publish decisions.<msg.id> { verdict, fired }
```

- **Why:** durable, decoupled, backpressure-friendly — the right shape for a real ops pipeline where
  alerts fan in from many sources.
- **Cost:** adds a queue-client dependency (NATS / Redis Streams / SQS). That is a new third-party
  module and must be **quarantined behind an adapter interface**, the same discipline as
  `model/claude` — the core stays stdlib-only, the queue client lives in one package.
- **Open choice:** which broker. NATS is the lightest to run locally for a demo; SQS if the target is
  AWS-native.

### B. HTTP webhook receiver

A small HTTP server that accepts POSTed alerts and decides synchronously per request.

```
POST /alert   (Alertmanager / PagerDuty / generic JSON)
  ─▶ parse alert → event text ─▶ Engine.Decide(event) ─▶ 200 { verdict, fired }
```

- **Why:** most monitoring stacks *push* alerts over HTTP; this is the lowest-friction integration for
  incident remediation specifically.
- **Cost:** an open listening port (an attack surface) and payload parsing per alert source. Both are
  ingress concerns, entirely outside the gate. Stdlib `net/http` — no new dependency.
- **Note:** "synchronous per HTTP request" is fine and does not violate invariant 9 — the HTTP handler
  is the caller waiting on the synchronous gate, not an async callback that gates. The alert *arrived*
  asynchronously; the decision it triggers is synchronous.

## Status

Explained and deferred (this doc). Build order unchanged: finish egress (verifiable release events)
and real actuators first; then add a listener — message-queue consumer leaning primary, HTTP webhook
receiver second — each as a quarantined adapter in front of the existing decide seam. Neither changes
the trust model: events are untrusted Input, the gate stays synchronous and sole, egress stays
off-path.
