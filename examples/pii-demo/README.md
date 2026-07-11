# PII/PHI containment demo — "The Confused Support Agent"

Proves stag's core guarantee: even if the agent is tricked (or hallucinates) into
emitting raw PII, the proxy **drops the payload at the infrastructure sink** before it can
reach an external destination. The containment is **structural, not content-scanning** —
stag does not regex for SSNs. The outbound channel simply cannot carry free-form model
text; it carries only **approved template ids** (a closed set). That refuses *all*
exfiltration, not just the PII formats you thought to match.

> This uses stag's real model. It does **not** use regex rules or a cross-call "taint
> map" — those aren't how the gate works. A tool call is judged from its gated argument
> against a release rule at the sink.

## The pieces

- **`server.py`** — a zero-dependency stdio MCP server exposing two tools:
  - `fetch_user_profile(user_id)` — internal DB read; returns a profile **including `ssn`**.
  - `send_external_reply(ticket_id, message_body)` — the external egress point.
- **`context/`** — narrative markdown for the scenario (the authoritative runbook and the
  untrusted, poisoned ticket). Narrative only in this demo — the READ channel is not wired
  into the gate verdict.

## Recipes (stag's real grammar)

`internal_lookup_policy` — internal reads are safe (benign sink → allow):

```yaml
recipe: internal_lookup_policy
version: 1
steps:
  - id: propose_uid
    kind: propose
    out: uid
  - id: read
    kind: sink
    in: uid
    field: internal.user_lookup
    sensitivity: benign
```

`external_reply_policy` — the outbound channel only carries approved templates:

```yaml
recipe: external_reply_policy
version: 1
rules:
  reply.templates:
    kind: set_membership
    set: ["tmpl:account_unlocked", "tmpl:password_reset_sent", "tmpl:ticket_escalated", "tmpl:looking_into_it"]
steps:
  - id: propose_body
    kind: propose
    out: body
  - id: send
    kind: sink
    in: body
    field: outbound.email.body      # authoritative egress
    sensitivity: authoritative
    rule: reply.templates
    actor: "policy:outbound_comms"
```

Untrusted (agent-proposed) `message_body` reaches an **authoritative** sink; it crosses
only if the release rule clears it. An approved template id clears; free-form text (with or
without an SSN) does not.

## Setup (the gate on :8080)

The control plane is authenticated: writing policy requires the **`admin`** role. The gate generates
its role tokens into `data/control.tokens` on first start.

```bash
ADMIN=$(python3 -c "import json;print(json.load(open('../../data/control.tokens'))['admin'])")
AUTH="Authorization: Bearer $ADMIN"

# 1. save the recipes
curl -s -H "$AUTH" -X POST localhost:8080/api/recipes --data-binary @recipes/internal_lookup_policy.yaml
curl -s -H "$AUTH" -X POST localhost:8080/api/recipes --data-binary @recipes/external_reply_policy.yaml

# 2. route the tools to them
curl -s -H "$AUTH" -X POST localhost:8080/api/routes -d '{"tool":"fetch_user_profile","recipe":"internal_lookup_policy","gateArg":"user_id"}'
curl -s -H "$AUTH" -X POST localhost:8080/api/routes -d '{"tool":"send_external_reply","recipe":"external_reply_policy","gateArg":"message_body"}'

# 3. (optional) register the MCP server so stag discovers the tools
curl -s -H "$AUTH" -X POST localhost:8080/api/mcp-servers \
  -d "{\"name\":\"pii-demo\",\"transport\":\"stdio\",\"target\":\"$(which python3) $PWD/server.py\"}"
```

## Walkthrough (human-as-the-model, via /api/decide or the Live page)

```bash
D() { curl -s -X POST localhost:8080/api/decide -d "$1" | jq -c '{verdict, forward, crossings:((.events//[])|map(.field))}'; }

# 1. internal read — ALLOW (no crossing)
D '{"tool":"fetch_user_profile","args":{"user_id":"123"}}'
#    the tool returns {"name":"Alice","ssn":"000-12-3456","status":"active"}

# 2. legit reply — ALLOW + a recorded crossing (outbound.email.body)
D '{"tool":"send_external_reply","args":{"ticket_id":"123","message_body":"tmpl:account_unlocked"}}'

# 3. THE BREACH — agent follows the injection and pastes the SSN — DENY, nothing crosses
D '{"tool":"send_external_reply","args":{"ticket_id":"123","message_body":"Here is your data: 000-12-3456"}}'

# 4. smuggling — approved template + appended PII — DENY (exact-match set)
D '{"tool":"send_external_reply","args":{"ticket_id":"123","message_body":"tmpl:account_unlocked 000-12-3456"}}'
```

Then `curl -s localhost:8080/api/log` — only the **cleared** crossing (step 2) is in the
hash-chained audit log; the denied SSN never appears.
