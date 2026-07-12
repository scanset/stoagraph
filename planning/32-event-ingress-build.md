# 32 — Event ingress build: adapters, attribution, and the validation lane

Recorded 2026-07-12. Planning/13 fixed the seam (the listener lives outside the gate, async-in /
synchronous-gate, events are untrusted Input) and deferred the build. This doc un-defers it: the
concrete design for production event ingress, incorporating two ratified assumptions — **event
formats differ by source** (Azure Sentinel vs a k8s cluster vs a user request), and **an
unauthenticated event is untrusted input that may still kick off a validation workflow before
anything acts on it.** Builds on 13 (the seam), 25 (dispatch + sessions), 31 (control-plane roles).

## The governing rule: attribution upgrades routing, never content

Every event splits into an **envelope** (who delivered it, what it claims to be) and a **payload**
(its content). They get separate trust treatment, and the treatments never mix:

| | Attributed channel (verified) | Unattributed channel |
| --- | --- | --- |
| **Envelope** (source, type) | routable — may dispatch a definition directly | a hint — may only trigger validation |
| **Payload** (content) | **untrusted Input, always** | **untrusted Input, always** |

Channel attribution — an HMAC-signed webhook, an authenticated queue subscription, a k8s
serviceaccount, a console session — proves *who sent the event*, never that its contents are safe
to obey. The trap this table exists to prevent: an authenticated Sentinel alert **quotes
attacker-controlled strings** (log lines, URLs, process arguments). Attribution earns the envelope
routing rights; the payload rides to the model in `Input`, stamped untrusted, exactly like a RAG
chunk. There is **no mechanism at ingress to promote content** — promotion happens only where it
always has, at the sink, by a rule firing with a recorded release.

## The canonical envelope

Formats differ per source; the dispatcher sees one shape. Each source adapter normalizes into:

```go
type Event struct {
    ID         string            // adapter-derived: source-native id, else hash
    Source     string            // adapter name: "sentinel", "alertmanager", "user", ...
    Type       string            // normalized event type within the source
    ReceivedAt time.Time
    Attributed bool              // channel auth verified by the adapter
    AuthMethod string            // "hmac" | "queue-sub" | "session" | "none"
    Payload    json.RawMessage   // VERBATIM source content — never rewritten, never interpreted
}
```

Normalization is **mechanical** (field mapping), never semantic. Adapters parse fail-closed per the
house parse discipline: an unparseable or oversized body is dropped and recorded, never repaired.

## Per-source adapters (quarantine discipline)

One package per source, the same discipline as `model/claude` and the future queue client: the
core stays stdlib-only; each adapter owns its transport parsing and its channel-auth check.

- **`generic`** — JSON POST + shared-secret HMAC header. The reference adapter; built first.
- **`sentinel`** — Azure Sentinel / Logic Apps webhook shape; HMAC or bearer.
- **`alertmanager`** — the k8s lane (Alertmanager webhook format); in-cluster serviceaccount or HMAC.
- **`user`** — a request from the console or an API client; attribution = the authenticated session.
  An attributed *user* routes directly, but their text is payload: untrusted Input (a request is not
  an instruction; the trust-position invariant is unchanged).

An adapter does three things: verify channel auth → normalize the envelope → hand to the
dispatcher. It holds no policy, knows no label vocabulary, and never reads payload semantics
(invariant: the listener earns no trust, Planning/13 rule 3).

## Two-lane dispatch

```
             ┌─ attributed + matched definition ──▶ LANE 1: direct dispatch
  Event ─────┤                                       (existing seam: definition → session →
  (envelope) │                                        recipe + tools + context; payload = Input)
             └─ unattributed, or definition ──────▶ LANE 2: validation workflow
                says validate                        (gated read-backs verify the claim;
                                                      the VERIFIED FACT re-dispatches)
  no match at all ──▶ drop + record (fail closed)
```

**Lane 1 — direct.** The existing dispatch seam (Planning/25): match the definition, bind the
session, run. Nothing new except that the trigger is now a wire instead of the simulate button.

**Lane 2 — validation.** The novel piece, and it needs **no new trust machinery.** An
unattributed event claiming "incident 4471 in Sentinel" is neither believed nor dropped: it
triggers a *validation recipe* whose only tools are **authenticated read-backs** through the
gate's own downstreams — does the authoritative system actually have incident 4471, and what does
*its* record say? The actionable input for the real workflow is the fact fetched from the
authoritative source, not the event that claimed it. The event is demoted to a hint. Promotion
runs through the existing rule kinds (`signed_equality` / `set_membership` against the lookup
result), the workflow is gated and recorded like any other, and on success the adapter emits a
**synthetic attributed event** (`source: "validated:<recipe>"`) that re-enters the same map. This
is the sink doctrine applied at the front door: trust is re-derived at an authoritative source,
never carried by the message.

## Lane 2 is a cost amplifier — bound it before anything smart runs

An open lane that turns anonymous POSTs into workflows is a token-burning DoS surface. Ordering is
load-bearing — every deterministic check runs before any model or tool call:

1. schema parse (fail-closed) and size cap;
2. dedup: `(source, ID)` plus payload hash, within a replay window;
3. per-source rate cap and a global queue-depth ceiling;
4. validation recipes are read-only and **preferably model-free** — a `signed_equality` check
   against an API response needs no proposer;
5. a per-source validation budget per window; exhausted → drop + record.

Unattributed ingress must cost the attacker more than it costs us.

## The ingress log

Every arrival is recorded **regardless of disposition** — the same discipline as the read log:

```
{ event_id, source, type, attributed, auth_method, matched_definition|null,
  disposition: dispatched | validated | dropped(reason), ts }
```

This extends the audit story to the front door — every event that arrived and what we did about
it — and makes unmatched events visible instead of silently absent. Hash-chain it like the
decision log; it is evidence, not telemetry.

## Event map extension

The map gains an attribution dimension; defaults fail closed:

```json
{
  "id": "sentinel-incident",
  "match": { "source": "sentinel", "type": "incident" },
  "require_attribution": true,
  "on_unattributed": "validate",
  "validation_recipe": "verify_sentinel_incident",
  "recipe": "incident_remediation",
  "tools": ["get_incident", "isolate_host"],
  "context": ["runbooks"]
}
```

- `require_attribution: true` + attributed event → lane 1.
- `on_unattributed: "validate"` → lane 2; `"drop"` (the default) → drop + record.
- A source with no matching definition at all → drop + record. Nothing is pre-trusted.

## Placement and invariants (restated, unchanged)

The listener lives on the **orchestrator** side, in front of the dispatch seam — never in the
gate. It authenticates to the control plane with the **`dispatch`** role only (Planning/31): it
can bind sessions and poll; it **cannot approve**. Invariant 9 holds: ingress is async, the gate
stays synchronous and sole; a webhook cannot gate. Worst case for a compromised adapter is
unchanged from Planning/13 — it delivers a bad event, which is the untrusted-input case the gate
bounds — plus, now, a bounded validation spend (the caps above are what keep that corollary true).

## Build ladder

| Unit | Builds | Pass bar |
| --- | --- | --- |
| **I1** | envelope + ingress log, observe mode | events arrive, normalize, record; nothing dispatches |
| **I2** | webhook receiver + `generic` HMAC adapter + lane-1 dispatch | one attributed event flows end-to-end through the existing seam; unmatched drops are recorded. **The first domino: "the life of one event" becomes demonstrable** |
| **I3** | named adapters: `sentinel`, `alertmanager`, `user` | three real formats → one envelope; normalization table-driven, fail-closed |
| **I4** | the validation lane | a forged unattributed event triggers validation; a real claim verifies and dispatches; a false claim dies with a recorded reason |
| **I5** | queue consumer transport (broker TBD; NATS leaning for local demo) | same envelope, same map, quarantined client package |
| **I6** | hardening: dedup, replay window, rate + budget caps | the lane-2 cost bound holds under a flood test |

Ladder order is deliberate: I1–I2 unlock the end-to-end demo and the explainer narrative; the
validation lane lands only after the boring plumbing has proven the envelope.

## The honest ceiling

- Validation attests only what an authoritative API can attest — that a claim checks out against
  the system of record. A **validated event's payload is still untrusted content** inside the
  workflow it dispatches; validation upgrades the trigger, not the text.
- Attribution of a user is not trust of a user's words. The `user` adapter routes on identity;
  the request rides as Input.
- Lane 2 does not make unattributed ingress free of cost — it makes the cost bounded and paid
  mostly by the sender. A determined flood still burns the budget cap; that is the designed loss.

## Open decisions

1. HMAC scheme per source: one shared-secret header (generic) vs source-native signatures
   (GitHub-style `X-Hub-Signature-256`) where the source offers them.
2. Broker for I5: NATS (lightest local) vs SQS (AWS-native target) vs Redis Streams.
3. Where the lane-2 budget cap lives: in the adapter (per-source, simple) or the dispatcher
   (global view, sees all sources).
4. Synthetic-event provenance naming: `validated:<recipe>` vs `verified:<source>` — pick one and
   keep it forever; it appears in the ingress log and the audit story.
