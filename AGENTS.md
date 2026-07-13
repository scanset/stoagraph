# AGENTS.md — orientation for AI agents working in this repo

**StoaGraph**: verifiable control for AI agents. An agent proposes; a deterministic gate disposes, with
no model in the decision path. This repo is the whole product, Apache-2.0, with no held-back edition.
New here as a human? Start with [README.md](README.md); the threat model is [SECURITY.md](SECURITY.md).

## The one thing to hold

Two processes, and the split IS the product:

- **`stag`** is the GATE: the deterministic kernel, the MCP gating proxy, policy, audit, and approvals.
  It holds NO model and NO API keys.
- **`harness`** is the ORCHESTRATOR: the dispatcher, the agent loop, and the model connections. It holds
  the keys.

The dependency runs ONE WAY: `harness -> stag`, never the reverse. This is enforced, not intended.
`stoa-kernel/architecture_test.go` fails the build if any gate package, or either gate binary, imports
orchestrator code. If that test is red, the product's central claim (the gate cannot reach your keys) is
false and the build must not ship. Do not weaken it to make something compile.

## The load-bearing invariant

No untrusted value reaches an authoritative sink without BOTH a gate verdict AND a recorded ReleaseEvent.
A path that violates this is a product-defining bug, not a mere test failure. Trust classes are an
ordered scalar (untrusted < caller < authoritative) compared at the sink; there is no taint propagation
through the model. Every proposal is presumed untrusted and its trust is re-derived at the sink from the
policy rule that fires.

## Layout

```
stoa-kernel/   one Go module, the whole backend
  stag/        the GATE         (kernel, policy, proxy, auth, audit, approvals)
  harness/     the ORCHESTRATOR (dispatch, agent loop, models)
  cmd/         stag-serve, stag-proxy, harness-serve, stag-tools, stoagraph, harness, healthcheck
  architecture_test.go   the one-way-dependency guard
frontend/      the console (a modified Next.js), talking to both backends
config/        event map + model config        data/  runtime state (gitignored)
docs/          recipe authoring, routes, the MCP proxy, docker, development
examples/      custom-tool (start here), local-tools, oauth-profiles
tools/         build, up, down, check, hygiene, sbom, find, index
```

## Working here

```bash
tools/build.sh      # build every binary into stoa-kernel/bin/
tools/up.sh         # run the whole product locally
tools/check.sh      # gofmt, vet, test, ARCHITECTURE, typecheck, index, hygiene (run before every commit)
tools/find.sh <kw>  # keyword code index, not grep
```

- Run Go commands from `stoa-kernel/` (`go build ./...`, `go test ./...`, `go vet ./...`).
- Every source file carries a `// file-kw:` marker that feeds `tools/find.sh` and `INDEX.md`; add one to
  any new file or `tools/check.sh` fails. `INDEX.md` is generated: do not hand-edit it, run `tools/index.sh`.
- The frontend is a modified Next.js. Read [frontend/AGENTS.md](frontend/AGENTS.md) before touching it.

## Style

Plain and grounded, no hype. Verify, do not assert: build- and run-verify, and show the evidence;
"compiles" is not "behavior-correct." State the honest ceiling. The product's credibility rests on not
over-claiming, so read the non-goals in [SECURITY.md](SECURITY.md) as carefully as the guarantees.
