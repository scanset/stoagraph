#!/usr/bin/env bash
# tools/gen-env.sh — mint the four control-plane role secrets into .env (gitignored) for compose.
#
# compose injects these per-service, so each container gets ONLY what it is entitled to and NO
# container mounts a tokens file. In particular the orchestrator never receives `approve`: it waits
# on a human decision, it can never forge one.
set -euo pipefail
cd "$(dirname "$0")/.." || exit 1

[ -f .env ] && { echo ".env already exists — delete it to rotate the control plane"; exit 1; }

gen() { head -c32 /dev/urandom | od -An -tx1 | tr -d ' \n'; }
cat > .env <<ENV
# StoaGraph control-plane secrets. GITIGNORED. Rotate by deleting this file and re-running.
STAG_ADMIN_TOKEN=$(gen)
STAG_APPROVE_TOKEN=$(gen)
STAG_DISPATCH_TOKEN=$(gen)
HARNESS_OPERATOR_TOKEN=$(gen)

# The orchestrator container runs as this uid so it can read your private config/models.json.
HOST_UID=$(id -u)
HOST_GID=$(id -g)

# PROVIDER KEYS (optional but preferred): reference them from config/models.json via "apiKeyEnv"
# instead of embedding a key in the file. The gate never sees either way.
# ANTHROPIC_API_KEY=
# OPENAI_API_KEY=
ENV
chmod 600 .env

echo "wrote .env (0600). Paste these into the console sidebar:"
grep -E 'ADMIN|OPERATOR' .env | sed 's/^/  /'
echo
echo "  APPROVE releases a held action — keep it for when you mean it."
echo "  DISPATCH is the orchestrator's; you never type it anywhere."
