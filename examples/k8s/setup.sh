#!/usr/bin/env bash
# Wire the k8s-test scenario into StoaGraph: save the recipe(s), register the k8s-ops MCP
# server (discovers its tools — no cluster needed for that), and route the read tools.
# Requires stag-serve running (default :8080). Re-runnable (upserts).
set -euo pipefail
API="${API:-http://localhost:8080}"
DIR="$(cd "$(dirname "$0")" && pwd)"
PY="$(command -v python3)"

# Control-plane auth (Planning/31): every write below is `admin`-gated. Take the token from the file
# stag-serve generated on first start (override with STAG_ADMIN_TOKEN).
TOKENS="${TOKENS:-$DIR/../../data/control.tokens}"
ADMIN="${STAG_ADMIN_TOKEN:-}"
if [ -z "$ADMIN" ] && [ -f "$TOKENS" ]; then
  ADMIN="$("$PY" -c "import json,sys; print(json.load(open('$TOKENS'))['admin'])")"
fi
if [ -z "$ADMIN" ]; then
  echo "no admin token: start stag-serve once to generate $TOKENS, or set STAG_ADMIN_TOKEN" >&2
  exit 1
fi
# curl with the admin bearer on every call
curl() { command curl -H "Authorization: Bearer $ADMIN" "$@"; }

echo "== recipes =="
for r in "$DIR"/recipes/*.yaml; do
  curl -s -X POST "$API/api/recipes" --data-binary @"$r" | jq -c '{name, valid, error}'
done

echo "== register k8s-ops MCP server (stdio -> python3 server.py) =="
curl -s -X POST "$API/api/mcp-servers" \
  -d "{\"name\":\"k8s-ops\",\"transport\":\"stdio\",\"target\":\"$PY $DIR/server.py\"}" \
  | jq -c '{name, tools:(.tools|map(.name)), discoverError}'

echo "== routes: read tools -> k8s_read_policy (auto-allow) =="
for tool in get_pods get_deployments get_pod_logs describe_pod get_nodes get_events; do
  curl -s -X POST "$API/api/routes" \
    -d "{\"tool\":\"$tool\",\"recipe\":\"k8s_read_policy\",\"gateArg\":\"namespace\"}" >/dev/null
done

echo "== routes: mutating tools -> tiered recipes (scale is multi-arg + approval; rest single-arg) =="
route() { curl -s -X POST "$API/api/routes" -d "{\"tool\":\"$1\",\"recipe\":\"$2\",\"gateArg\":\"$3\"}" >/dev/null; }
# Stage 5: scale routes to the APPROVAL policy — prod (any count) and dev/staging 6..20 ESCALATE
# into the approval queue; a human approve mints a signed release that the retry passes. (The
# plain multi-arg policy k8s_scale_multi_policy is still loaded as a simpler reference.)
route scale_deployment   k8s_scale_approval_policy  namespace,replicas,approval_token
route restart_deployment k8s_restart_policy      namespace  # dev,staging auto / prod escalate / else deny
route delete_pod         k8s_delete_pod_policy   namespace  # dev,staging auto / else deny
route delete_deployment  k8s_delete_deploy_policy namespace # always escalate (destructive)
route delete_namespace   k8s_delete_ns_policy    namespace  # hard deny (catastrophic)

curl -s "$API/api/routes" | jq -r '.[]|select(.recipe|startswith("k8s_"))|"  \(.tool) -> \(.recipe) (\(.gateArg)) valid=\(.valid)"'

echo "== READ channel: register the k8s-kb context provider (Planning/30) =="
# The gate proxies THIS http endpoint (kbserve) and stamps its output untrusted. The embedder lives
# in kbserve (downstream), so the gate stays model-free. Run kbserve separately:
#   go run ./cmd/kbserve   (from event_harness/, serves :8095)
KB_URL="${KB_URL:-http://localhost:8095/context}"
curl -s -X POST "$API/api/providers" \
  -d "{\"name\":\"k8s-kb\",\"kind\":\"http\",\"config\":\"{\\\"url\\\":\\\"$KB_URL\\\"}\",\"enabled\":true}" \
  | jq -c '{name, kind, enabled}'

echo
echo "done. Drive it with:  stag-proxy -downstream k8s-ops -http :8091   (run from the release/ root)"
