#!/usr/bin/env bash
# tools/demo.sh — load the containment demo into a running (empty) gate.
#
# The gate ships EMPTY on purpose: a security control must not arrive already permitting something you
# never authored. This script is the "yes, I meant it" step — it authors the policy and wires the tools.
#
# What you are about to watch:
#
#   fetch_user_profile(123)   -> ALLOWED. Returns Alice's record, INCLUDING her SSN.
#   send_external_reply(...)  -> the egress point. It may carry ONLY one of four approved template ids.
#
# So the agent can read the SSN and cannot send it — and NOT because anything scans for SSNs. No
# free-form value can cross at all. A jailbroken, prompt-injected or simply confused model can propose
# the exfiltration all day; the gate will not release it. Containment is structural.
set -euo pipefail
cd "$(dirname "$0")/.." || exit 1

GATE="${GATE:-http://localhost:8080}"
curl -sf -m 2 -o /dev/null "$GATE/health" || { echo "the gate is not up — 'docker compose up -d' or 'tools/up.sh'"; exit 1; }

# the admin role authors policy (from .env under docker, or data/control.tokens on the host)
if [ -n "${STAG_ADMIN_TOKEN:-}" ]; then ADMIN="$STAG_ADMIN_TOKEN"
elif [ -f .env ]; then ADMIN=$(grep '^STAG_ADMIN_TOKEN=' .env | cut -d= -f2)
elif [ -f data/control.tokens ]; then ADMIN=$(python3 -c "import json;print(json.load(open('data/control.tokens'))['admin'])")
else echo "no admin token: run tools/gen-env.sh (docker) or start stag-serve once (host)"; exit 1; fi
AUTH="Authorization: Bearer $ADMIN"

# Where the downstream lives. Inside compose the gate resolves the service name; on the host it is local.
if docker compose ps --format '{{.Service}}' 2>/dev/null | grep -q '^pii-demo$'; then
  TARGET="${TARGET:-http://pii-demo:9000/mcp}"
else
  TARGET="${TARGET:-http://localhost:9000/mcp}"
  echo "note: no pii-demo container — start one:  stoa-kernel/bin/example-pii -http :9000 &"
fi

echo "== policy =="
for r in examples/pii-demo/recipes/*.yaml; do
  curl -s -H "$AUTH" -X POST "$GATE/api/recipes" --data-binary @"$r" | python3 -c "import json,sys;d=json.load(sys.stdin);print('  ' + d['name'] + ': valid=' + str(d['valid']))" 
done

echo "== the tool server (streamable HTTP, so a containerised gate can reach it) =="
curl -s -H "$AUTH" -X POST "$GATE/api/mcp-servers" \
  -d "{\"name\":\"pii-demo\",\"transport\":\"http\",\"target\":\"$TARGET\"}" \
  | python3 -c "import json,sys;d=json.load(sys.stdin);print('  ' + d['name'] + ' -> ' + str([t['name'] for t in d.get('tools',[])]) + '  err=' + str(d.get('discoverError')))" 

echo "== routes: which policy governs which tool =="
curl -s -H "$AUTH" -X POST "$GATE/api/routes" -d '{"tool":"fetch_user_profile","recipe":"internal_lookup_policy","gateArg":"user_id"}' >/dev/null
curl -s -H "$AUTH" -X POST "$GATE/api/routes" -d '{"tool":"send_external_reply","recipe":"external_reply_policy","gateArg":"message_body"}' >/dev/null
curl -s -H "$AUTH" "$GATE/api/routes" | python3 -c "import json,sys;[print('  ' + r['tool'].ljust(20) + ' -> ' + r['recipe'] + '  valid=' + str(r['valid'])) for r in json.load(sys.stdin)]" 

cat <<TRY

== try it (no model or API key needed) ==

  # the agent reads the record — ALLOWED (it is an internal, benign read)
  curl -s -H "$AUTH" -X POST $GATE/api/decide \\
    -d '{"tool":"fetch_user_profile","args":{"user_id":"123"}}' | python3 -m json.tool | head -5

  # it tries to email the SSN out — DENIED, and never reaches the tool
  curl -s -H "$AUTH" -X POST $GATE/api/decide \\
    -d '{"tool":"send_external_reply","args":{"message_body":"Your SSN is 000-12-3456"}}' | python3 -m json.tool | head -5

  # an approved template — ALLOWED
  curl -s -H "$AUTH" -X POST $GATE/api/decide \\
    -d '{"tool":"send_external_reply","args":{"message_body":"tmpl:account_unlocked"}}' | python3 -m json.tool | head -5

Or open the console (:3000) and watch it in Live. With a model connected, dispatch an event and watch a
real agent try — and fail — to get the SSN out.

For the real-infrastructure demo (a live Kubernetes cluster): bash examples/k8s/setup.sh
TRY
