# Stoa Graph — assurance console

The live-testing console for the StoaGraph gating MCP proxy. Propose a tool call, watch the deterministic
gate allow / deny / escalate it, and see the provable loop and the signed audit record — all against the real
runtime (github.com/scanset/StAG).

Next.js 16 (App Router) + React 19 + Tailwind v4. The page (`app/page.tsx`) is a client component that drives
every panel from the backend API (`app/lib/api.ts`); nothing is mocked.

## Pages

- **Live** (`/`) — propose a tool call and watch it gate: the Decision stream, the Detail with the
  `sense → reason → decide → act → prove` provable loop and a plain-language reason, the Signed-record panel
  (`Verify chain`), and session stats.
- **Recipes** (`/recipes`) — the authoring page: a YAML editor with **live linter feedback** (errors with
  line numbers) and a **tier preview** (each label → auto / escalate / benign / deny), a list of stored
  recipes, and save / delete. Recipes are validated by the real StoaGraph linter server-side; an invalid
  recipe is never saved.
- **Adapters** (`/adapters`) — configure both proxy channels: **MCP servers** (adding one discovers its tools;
  each tool call is gated), **context providers** (the read channel; everything they return is untrusted +
  recorded), and **route bindings** (tool → recipe → gated arg — the gate's source of truth, each shown
  resolved or unresolved).

## Running it (wired to the backend)

The console talks to the StoaGraph HTTP API (`stag-serve`). Start both:

```bash
# 1. the backend (from the StAG repo)
cd /home/local/StructuralAssuranceGraph
go build -o /tmp/stag-serve ./harness/workspaces/stag/cmd/stag-serve
/tmp/stag-serve -addr :8080          # serves /api/decide|log|policies|health

# 2. the console (this repo)
cd /home/local/stoa-graph
npm run dev                          # http://localhost:3000
```

Open http://localhost:3000, type an argument value (or click an example chip), and hit **Decide**. Allowed
values (e.g. `hello`) forward and record a signed crossing; denied values (e.g. `rm -rf /`) are blocked and
never reach a tool.

## Configuration

The API base URL is `NEXT_PUBLIC_API_BASE` (default `http://localhost:8080`). The backend sends permissive
CORS for dev, so the browser calls it directly — no proxy needed. Set the env for a different backend:

```bash
NEXT_PUBLIC_API_BASE=http://localhost:8080 npm run dev
```

> Internal/dev use only: the API has no auth and permissive CORS. Do not expose it publicly without a token.
