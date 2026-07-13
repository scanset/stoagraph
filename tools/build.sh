#!/usr/bin/env bash
# tools/build.sh — build every binary into bin/.
#
# Always -o bin/. A bare `go build ./cmd/...` drops binaries beside the source, where a stray name
# can collide with a source directory (this repo already lost 19 .go files to exactly that). bin/ is
# gitignored as a directory, which cannot collide with anything.
set -euo pipefail
cd "$(dirname "$0")/.." || exit 1

mkdir -p stoa-kernel/bin
cd stoa-kernel
go build -o bin/ ./cmd/...

echo "built:"
for b in bin/*; do
  printf '  %-14s %s\n' "$(basename "$b")" "$(du -h "$b" | cut -f1)"
done
echo
echo "  stag-serve     the GATE control plane + policy API (:8080)"
echo "  stag-proxy     the GATE — MCP gating proxy / session daemon (:8091)"
echo "  harness-serve  the ORCHESTRATOR API — models, dispatch (:8092)"
echo "  kbserve        example context provider for the READ channel (:8095)"
echo "  harness        minimal CLI orchestrator"
