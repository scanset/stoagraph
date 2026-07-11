# k8s_test — stag against a real Kubernetes cluster

Same architecture as the mock demos (pii-demo, zt-ops), pointed at **real infra**. stag
doesn't change — you swap the downstream MCP server. This bundle is self-contained:

```
k8s_test/
├── server.py          # the k8s-ops MCP server (stdio) — shells out to LOCAL kubectl
├── recipes/           # the gating policies (v1: read-only auto-allow)
├── chart/             # a tiny helm chart: a sample workload to give the agent real objects
├── setup.sh           # wire server + recipes + routes into stag
└── README.md
```

## The shape

```
event_harness (model + KB runbooks)  ─MCP▶  stag-proxy (gate)  ─MCP▶  k8s-ops server ─kubectl▶  your cluster
```

- **k8s-ops server** runs LOCALLY and shells to `kubectl` (inherits your kubeconfig). List
  form only — a namespace like `"; rm -rf"` is an inert literal, never a shell exec.
- **v1 is READ-ONLY** (get pods/deployments/logs/nodes/events, describe pod) — safe to run
  against a live cluster to prove the wiring. Mutating/destructive tools get added and gated
  as the test case pushes.
- **Coarse single-arg gating**: the gated arg is `namespace`. Read recipe = benign = allow.

## 1. Launch a cluster + a workload

Any local cluster (kind / minikube / k3d). Then install the sample app into a few namespaces
so there's something to read (and later scale/restart):

```bash
helm install web ./chart -n dev     --create-namespace
helm install web ./chart -n staging --create-namespace --set replicaCount=3
helm install web ./chart -n prod    --create-namespace --set replicaCount=4
kubectl get pods -A            # sanity: pods running
```

## 2. Bring stag up + wire this scenario in

```bash
# from the repo root
go build -o /tmp/stag-serve  ./harness/workspaces/stag/cmd/stag-serve
go build -o /tmp/stag-proxy  ./harness/workspaces/stag/cmd/stag-proxy
/tmp/stag-serve -addr :8080 &

bash k8s_test/setup.sh         # saves recipe, registers k8s-ops, routes the read tools
```

## 3. Drive it

In the event_harness console (`:8090`), set the **stag-proxy command** to:

```
/tmp/stag-proxy -downstream k8s-ops -log deploy/mcp/proxy-decisions.jsonl
```

Then run an event, e.g. *"the web app in dev looks unhealthy — check the pods and recent
events."* The model proposes `get_pods` / `get_events`, each is gated (auto-allow, read-only)
and forwarded to `kubectl`, and the real cluster output streams back in the transcript.

## Next (as we push the test case)
- **Mutating tools** — `scale_deployment`, `restart_deployment`, `delete_pod`, … with tiered
  recipes (namespace `set_membership`: dev/staging auto, prod escalate; replica `numeric_range`;
  hard-deny on `delete_namespace`/`exec`).
- **KB runbooks** — wire `event_harness`'s `kb`+`bind` into the agent loop so the prompt is
  informed by runbooks ("high latency → scale service Y to N"). Runbooks inform the proposal;
  the proposal stays untrusted and gated.
- **Multi-arg gating** — the real motivation to let a recipe see several args of a call
  (namespace **and** replicas together).
