#!/usr/bin/env bash
# tools/down.sh — stop everything tools/up.sh started. Leaves data/ intact (it is your state).
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

for p in stag-serve stag-proxy kbserve harness-serve; do
  pkill -f "bin/$p" 2>/dev/null && echo "  stopped $p" || true
done
pkill -f 'next dev -p 3000' 2>/dev/null && echo "  stopped console" || true
# the gate spawns its downstream MCP server over stdio; it dies with the parent, but be sure.
pkill -f 'examples/k8s/server.py' 2>/dev/null || true
sleep 1

for port in 8080 8091 8095 8092 3000; do
  if curl -sf -m 1 -o /dev/null "http://localhost:$port" 2>/dev/null; then
    echo "  ! still listening on :$port"
  fi
done
echo "down. (data/ kept — delete it for a clean slate; tokens and the audit log regenerate)"
exit 0
