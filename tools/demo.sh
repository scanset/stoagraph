#!/usr/bin/env bash
# tools/demo.sh — wire the k8s example into a running gate, then show how to drive it.
#
# What this proves, end to end:
#   an EVENT arrives -> the dispatcher picks a recipe -> a session is bound ON THE GATE ->
#   the agent reads UNTRUSTED context through the gate -> every tool call is gated ->
#   a prod mutation ESCALATES and waits for a human.
#
# The agent never chooses its own recipe. A misroute cannot breach; it can only fail.
set -euo pipefail
cd "$(dirname "$0")/.." || exit 1

curl -sf -m 2 -o /dev/null http://localhost:8080/api/health || {
  echo "the gate is not up — run tools/up.sh first"; exit 1; }

kubectl get ns >/dev/null 2>&1 || {
  echo "no kubernetes cluster reachable."
  echo "  kind create cluster --name stoagraph"
  echo "  helm install web ./examples/k8s/chart -n prod --create-namespace --set replicaCount=4"
  exit 1; }

bash examples/k8s/setup.sh

echo
echo "now dispatch an incident — either in the console (:3000 -> Dispatch), or:"
cat <<'CURL'

  OP=$(python3 -c "import json;print(json.load(open('data/control.tokens'))['operator'])")
  curl -N -X POST http://localhost:8090/api/dispatch \
    -H "Authorization: Bearer $OP" -H 'Content-Type: application/json' -d '{
      "event": {"source":"pagerduty","title":"prod web returning 500s",
                "detail":"web in prod is throwing 500s to customers. Investigate, then remediate."},
      "model": "claude", "maxTurns": 8 }'

CURL
echo "watch for: reads ALLOW -> a prod mutation ESCALATES -> approve it in the console (Approvals)."
