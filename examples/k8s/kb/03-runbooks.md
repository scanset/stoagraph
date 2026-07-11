# Operational runbooks and change policy

Why the guardrails are shaped the way they are, and what the on-call agent should do.

## Change policy by blast radius

- **Reads are always safe** — `get_pods`, `get_deployments`, `get_events`, `describe_pod`,
  `get_pod_logs`, `get_nodes`. Always investigate first; reading never needs approval.
- **`dev` / `staging` mutations** — scaling and restarts are routine; act directly.
- **`prod` mutations** — customer-facing, so they **require human approval** (they escalate):
  - Restarting the prod `web` deployment escalates — it briefly drops capacity for live users.
  - Scaling prod escalates **regardless of the replica count** — even "scale to 3" is a prod change.
- **Deleting a whole namespace is never a remediation** — it is catastrophic and always denied.

## On-call playbook (SRE agent)

1. **Investigate** with the read tools; state the symptom (crash-loop? spike? one wedged pod?).
2. **Match the fix to the namespace:** in dev/staging, remediate directly; in **prod, propose the
   fix and let it go to approval** — do not try to route around the escalation.
3. **Prefer the least-blast-radius action** that resolves the symptom: delete one wedged pod before
   restarting the whole deployment; restart before scaling; scale before anything destructive.

## SLA

`prod` `web` carries a 99.9% availability target. Availability-affecting prod actions are logged and
require a named approver; that is the reason prod changes escalate rather than auto-apply.
