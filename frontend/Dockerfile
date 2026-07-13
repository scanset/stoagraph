# The console. One UI, two backends (the gate and the orchestrator) — it holds only the HUMAN's
# tokens, entered in the browser. It never receives the orchestrator's `dispatch` secret, and the
# orchestrator never receives `approve`.
ARG NODE_VERSION=22

FROM node:${NODE_VERSION}-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
# Drop `sharp` + its libvips blobs. Next pulls it in for next/image optimisation; this console uses
# next/image NOWHERE, so it is 33MB of dead weight — and LGPL-3.0, the only copyleft component in the
# whole product. Removing it keeps the licence surface permissive (MIT/BSD/Apache).
#
# Do NOT reach for `--omit=optional` here: Next's own native compiler (@next/swc-*) is ALSO an optional
# dependency, and omitting it breaks the build ("Turbopack is not supported on this platform"). Remove
# the one package, not the whole class.
#
# If you ever add next/image: delete the rm, and record the LGPL-3.0 obligation in the SBOM review.
RUN npm ci && rm -rf node_modules/sharp node_modules/@img

FROM node:${NODE_VERSION}-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# The API bases are baked at build time (Next inlines NEXT_PUBLIC_*). Compose passes the addresses
# the BROWSER will use — not the docker-network names, which a browser cannot resolve.
ARG NEXT_PUBLIC_API_BASE=http://localhost:8080
ARG NEXT_PUBLIC_HARNESS_API=http://localhost:8092
ENV NEXT_PUBLIC_API_BASE=$NEXT_PUBLIC_API_BASE
ENV NEXT_PUBLIC_HARNESS_API=$NEXT_PUBLIC_HARNESS_API
RUN npm run build

FROM node:${NODE_VERSION}-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production
# next standalone output keeps the runtime image small and needs no node_modules copy
COPY --from=build /app/.next/standalone ./
COPY --from=build /app/.next/static ./.next/static
COPY --from=build /app/public ./public
USER node
EXPOSE 3000
CMD ["node", "server.js"]
