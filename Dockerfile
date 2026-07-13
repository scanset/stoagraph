# One Dockerfile, four images. Pass --build-arg CMD=<binary>; compose does this per service.
#
#   stag-serve  stag-proxy      the GATE               — no model, no keys
#   harness-serve               the ORCHESTRATOR       — holds the model API keys
#   stag-tools                  the local tool server  — built with --target localtools
#
# They are separate CONTAINERS on purpose, and it is not an aesthetic choice. The control-plane
# secrets are per-role, and `approve` (which releases a held action) must never be reachable by the
# orchestrator — otherwise a compromised orchestrator approves its own escalations and the
# human-in-the-loop guarantee is theatre. Note the gate's HTTP role check does NOT save you there: a
# compromised orchestrator would not send `dispatch` to an approve route and accept the 401, it would
# send the `approve` token it was holding. Co-locating these processes in one container puts every
# secret on one filesystem and makes that impossible to prevent. See compose.yml and SECURITY.md.

ARG GO_VERSION=1.25

# ---------- build ----------
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

# deps first, so a code change does not re-download the module graph
COPY stoa-kernel/go.mod stoa-kernel/go.sum ./
RUN go mod download

COPY stoa-kernel/ ./

ARG CMD=stag-serve
# CGO off: the SQLite driver is modernc (pure Go), so these are static binaries that run on distroless
# with no libc.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${CMD}
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/healthcheck ./cmd/healthcheck

# The writable state dir must exist IN THE IMAGE, owned by the runtime user: Docker seeds a named
# volume from the image path on first mount, ownership included. Without this the volume lands
# root-owned and a nonroot service cannot create its own database.
RUN mkdir -p /out/data

# ---------- runtime: the LOCAL TOOL SERVER (build with --target localtools, CMD=stag-tools) ----------
# This is the one image that is deliberately NOT distroless, and the reason is honest: a tool server
# exists to RUN REAL COMMANDS, so the commands have to be in the image. `grep` cannot grep from an image
# that has no grep.
#
# That is not the hole it looks like. The containment is at the EXEC boundary, not in the absence of
# binaries: stag-tools never invokes a shell, refuses shell-shaped declarations at load, and substitutes
# every value into exactly one argv element — so a value cannot become a command no matter what sits on
# disk. Busybox being present buys an attacker nothing, because nothing ever parses a string as a program.
#
# What this container buys YOU is blast radius. The tools can only touch what you mount. Mount the
# workspace, mount it read-only, and mount NOTHING that holds a credential — see compose.yml.
#
# IT IS DELIBERATELY NOT THE LAST STAGE. `docker build` with no --target builds the LAST stage, and this
# one has a shell, git and ripgrep in it. When this sat at the bottom of the file, every caller that
# forgot --target (compose.override.yml, ci.yml, release.yml — all of them) silently shipped the GATE on
# alpine-with-a-shell while the comments, SECURITY.md and the README all said distroless. The safe image
# must be the one you get by default; an unsafe default is a claim waiting to go quietly false.
FROM alpine:3.20 AS localtools
RUN apk add --no-cache git ripgrep && adduser -D -u 65532 nonroot
WORKDIR /app

COPY --from=build --chown=nonroot:nonroot /out/app         /app/stag-tools
COPY --from=build --chown=nonroot:nonroot /out/healthcheck /app/healthcheck

USER nonroot:nonroot
ENTRYPOINT ["/app/stag-tools"]

# ---------- runtime (DEFAULT) ----------
# distroless: no shell, no package manager, nonroot. Nothing to pivot to if a binary is ever popped —
# which is also why the health probe is a static binary instead of `curl`.
#
# LAST ON PURPOSE, so that this — the hardened image — is what a bare `docker build` produces. Callers
# still pass --target runtime explicitly; this ordering is the belt to that braces. tools/check.sh
# asserts the built gate images actually have no shell, because "we said distroless" is not evidence.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app

COPY --from=build --chown=nonroot:nonroot /out/app         /app/stoagraph
COPY --from=build --chown=nonroot:nonroot /out/healthcheck /app/healthcheck
COPY --from=build --chown=nonroot:nonroot /out/data        /app/data

USER nonroot:nonroot
ENTRYPOINT ["/app/stoagraph"]
