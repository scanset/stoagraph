#!/usr/bin/env bash
# tools/gen-env.sh — mint the control-plane secrets into .env (gitignored) for compose.
#
# THREE secrets, injected per-service by compose so each container holds only what it is entitled to.
# The one that matters: the orchestrator never receives the console/approve secret, so it can never
# forge a human decision. It waits on a human; it cannot be one.
#
#   STAG_CONSOLE_TOKEN     the human's gate key — author policy + approve. (compose maps it to
#                          STAG_ADMIN_TOKEN and STAG_APPROVE_TOKEN.)
#   HARNESS_OPERATOR_TOKEN the human's orchestrator key — models + dispatch.
#   STAG_DISPATCH_TOKEN    MACHINE ONLY — the orchestrator process binds sessions with it. Never typed.
#
# Prefer `stoagraph up`, which does this and prints a one-click login link.
set -euo pipefail
cd "$(dirname "$0")/.." || exit 1

[ -f .env ] && { echo ".env already exists — delete it to rotate the control plane"; exit 1; }

gen() { head -c32 /dev/urandom | od -An -tx1 | tr -d ' \n'; }
cat > .env <<ENV
# StoaGraph control-plane secrets. GITIGNORED. Rotate by deleting this file and re-running.

# Your login (the console needs both; they ride behind one link — see: stoagraph console)
STAG_CONSOLE_TOKEN=$(gen)
HARNESS_OPERATOR_TOKEN=$(gen)

# Machine-only. The orchestrator PROCESS uses this to bind sessions; it CANNOT approve. Never typed.
STAG_DISPATCH_TOKEN=$(gen)

# The orchestrator container runs as this uid so it can read your private config/models.json.
HOST_UID=$(id -u)
HOST_GID=$(id -g)

# PROVIDER KEYS (optional but preferred): reference them from config/models.json via "apiKeyEnv"
# instead of embedding a key in the file. The gate never sees either way.
# ANTHROPIC_API_KEY=
# OPENAI_API_KEY=
ENV
chmod 600 .env

echo "wrote .env (0600)."
echo "  then: docker compose up -d   (or: stoagraph up, which also prints your login link)"
