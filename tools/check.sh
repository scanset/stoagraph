#!/usr/bin/env bash
# tools/check.sh — the full verification gate. One command; what CI runs.
#
#   gofmt · go vet · go test · ARCHITECTURE · frontend typecheck · repo hygiene
#
# The architecture test is not a formality. Merging the gate and the orchestrator into one Go module
# removed the module boundary that used to make "the gate holds no model and no keys" structurally
# true. stoa-kernel/architecture_test.go puts it back as an enforced rule: it fails if any stag/...
# package — or either gate BINARY — so much as imports harness/... . If that ever goes red, the
# product's central claim is false; it is not a style violation.
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

fail=0
step() { printf '\n== %s ==\n' "$1"; }
ok()   { printf '  ✓ %s\n' "$1"; }
no()   { printf '  ✗ %s\n' "$1"; fail=1; }

step "gofmt"
unformatted=$(gofmt -l stoa-kernel 2>/dev/null)
if [ -n "$unformatted" ]; then no "unformatted:"; echo "$unformatted" | sed 's/^/      /'; else ok "formatted"; fi

step "go vet"
if (cd stoa-kernel && go vet ./... 2>&1 | grep -v '^#' | grep .); then no "vet found problems"; else ok "vet clean"; fi

step "go test"
if (cd stoa-kernel && go test ./... 2>&1 | grep -vE '^ok|no test files' | grep .); then no "tests failed"; else ok "tests pass"; fi

step "ARCHITECTURE — the gate must not depend on the orchestrator"
if (cd stoa-kernel && go test -run TestGate . 2>&1 | grep -qE '^ok'); then
  ok "stag/ imports zero harness/ code (and neither gate binary links it)"
else
  no "ARCHITECTURE BREACH — the gate reaches orchestrator code. The 'no model, no keys' claim is FALSE."
fi

step "frontend typecheck"
if [ -d frontend/node_modules ]; then
  if (cd frontend && npx tsc --noEmit 2>&1 | grep .); then no "typecheck failed"; else ok "typecheck clean"; fi
else
  printf '  – skipped (run: cd frontend && npm ci)\n'
fi

step "repo hygiene"
if bash tools/hygiene.sh >/tmp/hygiene.$$ 2>&1; then
  ok "hygiene OK"
else
  no "hygiene FAILED:"; sed 's/^/      /' /tmp/hygiene.$$
fi
rm -f /tmp/hygiene.$$

echo
[ "$fail" -eq 0 ] && { echo "ALL CHECKS PASSED"; exit 0; }
echo "CHECKS FAILED"; exit 1
