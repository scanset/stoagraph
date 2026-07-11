# 25 — Event dispatch, the event map, and the session model

Recorded 2026-07-05. Companion to Planning/22 (topology), /23 (orchestration roadmap), /24 (stag-proxy).
This doc pins the design of the orchestration item /23 deferred: **event ingress → recipe dispatch** — and the
mediation change it depends on (**session→recipe binding**).

**Lineage.** stag's gate *is* Ratchet's "propose into constrained slots → deterministic gate decides"
pattern, lifted out of the console and made the product. The dispatcher now comes back the same way: Ratchet's
`internal/dispatch` routes an operator line to a flow (narrow → model proposes `flow_id` from an enum → pure
`Gate(flowID, confidence, validIDs)` → run). We port that shape to route an **event** to a **recipe**. It's
natural because it's the same machine we already trust.

## The load-bearing safety property

A **misroute cannot breach.** If the dispatcher binds an event to the wrong recipe, the gate still enforces
*that* recipe faithfully — a bad route wastes a turn, it does not let anything cross (Planning/23: "an
orchestration bug wastes a turn; a mediation bug lets PII cross"). This is why a model is allowed in the routing
path at all: the deterministic gate downstream is the backstop. Routing is untrusted; the gate is not.

## The event map — user-defined, because events are domain-specific

Events do NOT always arrive as clean types. A payload may be a webhook body, a queue message, a raw ticket — the
shape is the customer's domain, not ours. So the event→recipe mapping is **authored by the user**, the same way
recipes are, not hardcoded. It is a **definition/event map**: an ordered list of *definitions*, each a matcher
over the event plus the recipe it selects.

```
# event-map (user-authored; illustrative)
- def: pd-incident
  match: { source: pagerduty, "event.type": incident.triggered }   # deterministic predicate over the payload
  recipe: k8s_incident_policy
- def: refund-request
  match: { source: stripe, "event.type": charge.dispute.created }
  recipe: zt_refund_policy
- def: support-ticket                                               # messy / unstructured
  match: { source: zendesk }                                       # coarse predicate...
  route: model                                                     # ...then defer the recipe choice to the router
```

Two resolution modes, deterministic-first:
1. **Deterministic match** — a definition's predicate matches the payload → its recipe is selected. No model in
   the path. Most events want this: auditable, cheap, exact.
2. **Model route** — a definition marks `route: model` (or nothing matched): fall back to the Ratchet-style
   router — narrow the candidate recipes, the dispatch model proposes `{recipe_id (enum), confidence}`, the pure
   `Gate` accepts only an on-list recipe at non-low confidence, else no-dispatch.

The map is data the user owns (a file / a console tab), so it flexes to any domain without code changes. The
`Gate` is identical for both modes — a matched definition is just "confidence: high, deterministic."

## The dispatcher is a model role

Routing (mode 2) needs a model, so the harness's model registry gains a **role**, mirroring Ratchet's
`Models{Generate, Dispatch, Embed}` (`Dispatch` = "the small seat for the one constrained route decision," falls
back to `Generate` when unset):

- **proposer** — the agent-loop model that proposes tool calls (today's `claude` / `openai` entries).
- **dispatch** — a (usually small/cheap) model that makes the single constrained event→recipe decision.
- **embed** — optional, to narrow candidate recipes before the dispatch prompt (only needed at many recipes).

Concretely: a configured model may be *designated* the dispatch seat (like today's `keyPresent` models, plus a
role flag), defaulting to the proposer when unset. The dispatch decision is constrained-JSON, low-temp, one shot
— never freeform.

## The session model — session ≠ recipe

**A session is the durable unit of one dispatched governed run; a recipe is what it walks through.** They are
NOT 1:1, and the model must not assume they are, because:
- **Recipe→recipe** — a recipe can call another (static `goto_recipe` already composes to one hash today; a
  *runtime* transition to a different recipe is a future node kind). Either way it is the SAME session.
- **Subagent nodes** — a future node that dispatches a sub-agent runs that sub-agent *inside* the parent session
  (or as a child session linked to it). Its crossings belong to the same run's trace.

So the entity is:

```
Session {
  id
  entry_recipe          # what the dispatcher bound
  recipe_trace []        # recipes actually traversed (entry + any runtime transitions) — v1: just [entry]
  children []            # subagent sessions spawned by a node — v1: empty
  event_ref              # the event that triggered dispatch (provenance)
  ... audit head, status
}
```

**v1 scope: one recipe per session** — `recipe_trace` holds only the entry recipe, `children` is empty. But the
shape (a trace + a child list, not a single `recipe` field) is chosen now so multi-recipe and subagent sessions
drop in later without a schema change. Do NOT model it as `session.recipe` (a scalar) — that's the trap.

Binding: the dispatcher tells stag-proxy "this session is governed by recipe R." stag-proxy gates that session's
tool calls against R (v2 of the proxy — today it binds recipes per *tool*, globally; session→recipe lets two
dispatched agents run different policies over the same tools). The session id is the join key between
orchestration (who dispatched) and mediation (what's enforced).

## The partition (plan of record)

Three components, cleanly separable. Confirmed correct 2026-07-05.

| # | Component | Trust | Work |
| --- | --- | --- | --- |
| 1 | **event_harness** — dispatcher + ingress + dispatch-model | untrusted (commercial) | `dispatch/` package (port Ratchet's narrow→propose→`Gate`); the **event map** loader + deterministic matcher; the **dispatch model role**; an ingress endpoint/queue-subscriber that turns an event into a `Dispatch` call → `agent.Run` bound to the chosen recipe/session |
| 2 | **stag / stag-proxy** — session→recipe binding | trusted (open source) | standing daemon (a session to bind to; also clears the shared-log fork); a session carries its recipe; gate that session's calls against it. The **session** entity above lives here (mediation owns what's enforced) |
| 3 | **stoa-graph** — console | trusted (open source) | an **event-map editor** (author definitions → recipes, like the Routes tab keyed on event); a **dispatch log** view (event → recipe → verdict trail, over the existing audit log) |

## Sequencing (dependency order)

The dispatcher can *choose* a recipe, but there is nothing to *bind it to* until a session can carry a recipe.
So mediation goes first:

1. **stag-proxy v2** — standing daemon + session→recipe binding (+ the session entity). Unblocks dispatch;
   clears the log-fork debt. (Planning/24 v2.)
2. **event_harness dispatcher + ingress + event map + dispatch model** — the Ratchet port (this doc, §The event
   map / §dispatcher).
3. **stoa-graph** — event-map editor + dispatch log.

## v1 scope vs deferred

- **v1:** deterministic event map (predicate match) → session-bound dispatch of the existing agent loop through
  a session→recipe-bound stag-proxy; one recipe per session; model-route as a thin fallback.
- **Deferred:** runtime recipe→recipe transitions and subagent-node sessions (the `recipe_trace` / `children`
  fields exist but stay singular/empty); durable approval hold + durable session suspend (Redis/queue — "future
  us," per the approval-loop note); embedding-narrowed routing (only needed at many recipes).
