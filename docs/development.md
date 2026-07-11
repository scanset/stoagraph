# Development

Everything runs from the repo root. There is one Go module (`stoa-kernel`) and one frontend
(`frontend`); each binary boots with **zero flags** because every default path is relative to this root.

## The tools

| | |
|---|---|
| `tools/build.sh` | Build all binaries into `stoa-kernel/bin/`. |
| `tools/up.sh` | Bring up the gate, the orchestrator, and the console. Prints your control-plane tokens. |
| `tools/down.sh` | Stop everything. Leaves `data/` intact. |
| `tools/check.sh` | **The gate for a change:** gofmt · vet · test · architecture · typecheck · hygiene. |
| `tools/hygiene.sh` | Repo invariants that fail silently if nobody looks. |
| `tools/demo.sh` | Wire the k8s example into a running gate and show how to fire an incident. |

```bash
tools/build.sh && tools/up.sh      # http://localhost:3000
tools/check.sh                     # before you commit
tools/down.sh
```

## Layout, and the one rule

```
stoa-kernel/
  stag/      the GATE          — kernel, policy, proxy, auth, audit, approvals
  harness/   the ORCHESTRATOR  — dispatch, agent loop, models (the API KEYS live here)
  cmd/       the binaries; each ships as its own container
```

**The dependency runs one way: `harness/` → `stag/`, never the reverse.**

This is the product's central claim ("the gate holds no model and no keys"), and merging the two into a
single Go module removed the module boundary that used to make it structurally true. So it is now an
enforced test. `stoa-kernel/architecture_test.go` fails if any `stag/...` package — or either gate
*binary* — so much as imports `harness/...`.

If that test goes red, it is not a style violation. The claim is false and the build must not ship.

## Ports and roles

| Service | Port | What it is |
|---|---|---|
| `stag-serve` | 8080 | The gate's control plane: policy, approvals, audit. No model, no keys. |
| `stag-proxy` | 8091 | The gate's MCP proxy. Sessions are bound to a recipe here. |
| `harness-serve` | 8090 | The orchestrator's API. **Holds the model keys.** |
| `kbserve` | 8095 | Example context provider (the READ channel's downstream). |
| console | 3000 | One UI, talking to both backends. |

The console holds the **human's** tokens (`admin` / `approve` for the gate, `operator` for the
orchestrator). The orchestrator *process* holds only `dispatch` — it can bind sessions and poll for an
approval decision, but it **can never approve**. Do not "fix" a 401 by handing it a stronger token; that
401 is the human-in-the-loop working.

## Paths

| | |
|---|---|
| `data/` | **All runtime state** — `config.db`, **`recipes/`** (the gate reads *and writes* it), `control.tokens`, `approval.key`, audit logs. **Gitignored**; the binaries create it. `rm -rf data` for a clean slate. |
| `config/` | `event_map.json`, and `models.json` (your keys — gitignored; copy `models.example.json`). |
| `examples/` | `k8s` (a real cluster), `pii-demo`, `zt-ops`, `scratch`. **Each example carries its own recipes**, which its `setup.sh` loads into the gate. |

A fresh instance boots **empty** — 0 recipes, 0 routes, 0 policies. The recipe store is runtime state,
not shipped content, so nothing is trusted until you author or load it. `stag-serve -recipe <file>` will
seed one policy + route on first run if you want a one-liner demo.

## Why `tools/hygiene.sh` exists

Both bugs it catches actually happened in this repo, and **neither announced itself**:

1. A `.gitignore` line meant for a compiled binary (`/stoa-kernel/harness`) also matched the
   **orchestrator's source directory** and silently ignored 19 `.go` files. `go build` kept working
   locally; only a fresh `git clone` would have been broken — i.e. we'd have learned from the first
   person who tried to use it.
2. A `models.json` full of live API keys stayed **tracked** because `.gitignore` does nothing to a file
   that is *already committed*. It was pushed.

So: never add an ignore pattern that is a bare name colliding with a source directory (build to `bin/`),
and remember that ignoring a tracked file requires `git rm --cached`. `tools/hygiene.sh` checks both,
plus committed binaries. Run it in CI.

## Testing notes

- `go test ./...` from `stoa-kernel/` covers kernel, policy linter, proxy, auth, and the architecture rule.
- The k8s example test (`cmd/stag-proxy/e2e_test.go`) skips cleanly when `python3` or the example server
  is absent — but the path is **relative**, so it actually runs in CI. (It used to be an absolute path,
  which meant it silently skipped everywhere except one laptop.)
- Auth tests assert the load-bearing negative: `TestDispatchCannotApprove`. If it ever passes a
  `dispatch` token to an approve route, human-in-the-loop is gone.
