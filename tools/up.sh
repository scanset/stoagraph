#!/usr/bin/env bash
# tools/up.sh — bring the whole product up locally.
#
#   stag-serve  :8080   the GATE — policy, approvals, audit. No model. No keys.
#   stag-proxy  :8091   the GATE — MCP gating proxy; sessions bound to a recipe.
#   kbserve     :8095   example context provider (the READ channel's downstream).
#   harness-serve :8090 the ORCHESTRATOR — models (KEYS live here), dispatch.
#   console     :3000   the one console, talking to both backends.
#
# Order matters: stag-serve GENERATES data/control.tokens on first start, and everything else only
# READS them (a consumer must never invent a secret nobody else knows). So the gate boots first.
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

BIN=stoa-kernel/bin
[ -x "$BIN/stag-serve" ] || { echo "binaries missing — run tools/build.sh"; exit 1; }
mkdir -p logs   # the binaries create data/ themselves (a container has nobody to mkdir for it)

wait_for() { # url, name
  for _ in $(seq 1 40); do curl -sf -m 1 -o /dev/null "$1" && { printf '  ✓ %s\n' "$2"; return 0; }; sleep 0.25; done
  printf '  ✗ %s did not come up — see logs/\n' "$2"; return 1
}

echo "== the GATE =="
"$BIN/stag-serve" -addr :8080 >logs/stag-serve.log 2>&1 &
wait_for http://localhost:8080/api/health "stag-serve :8080"

"$BIN/stag-proxy" -downstream "${DOWNSTREAM:-k8s-ops}" -http :8091 >logs/stag-proxy.log 2>&1 &
wait_for http://localhost:8091/health "stag-proxy :8091 (daemon)" || \
  echo "    (no downstream registered yet? run tools/demo.sh)"

echo "== the READ channel =="
"$BIN/kbserve" -addr :8095 >logs/kbserve.log 2>&1 &
wait_for http://localhost:8095/health "kbserve :8095"

echo "== the ORCHESTRATOR =="
if [ ! -f config/models.json ]; then
  echo "  ! config/models.json missing — copy config/models.example.json and add a key."
  echo "    (the GATE needs none of this; only the orchestrator does)"
fi
"$BIN/harness-serve" -addr :8090 >logs/harness-serve.log 2>&1 &
wait_for http://localhost:8090/health "harness-serve :8090"

echo "== the CONSOLE =="
if [ -d frontend/node_modules ]; then
  (cd frontend && npx next dev -p 3000 >../logs/console.log 2>&1 &)
  wait_for http://localhost:3000 "console :3000"
else
  echo "  – skipped (cd frontend && npm ci)"
fi

echo
echo "control-plane tokens (data/control.tokens) — paste these into the console sidebar:"
if [ -f data/control.tokens ]; then
  python3 - <<'PY'
import json
t = json.load(open("data/control.tokens"))
print(f"  gate token         (admin)    {t['admin']}")
print(f"  gate token         (approve)  {t['approve']}   <- releases an escalation. HUMANS ONLY.")
print(f"  orchestrator token (operator) {t['operator']}")
print(f"  dispatch                      (held by the orchestrator process; it can never approve)")
PY
fi
echo
echo "  console  http://localhost:3000     stop with: tools/down.sh"
