# The `web` deployment

`web` is the customer-facing web application — the only workload in this cluster. It runs as a
Kubernetes Deployment named `web` in each namespace (`dev`, `staging`, `prod`), fronted by a Service.

- **Baseline size:** 2 replicas per namespace. It is horizontally scalable — more replicas absorb
  more traffic.
- **Health:** a healthy `web` pod is `Running` and `Ready`. `CrashLoopBackOff`, repeated restarts, or
  `0/1 Ready` mean the app is failing to start or crashing under load.
- **Common incidents:**
  - *Traffic spike* → pods saturate; the fix is to **scale up** (more replicas).
  - *Bad rollout / config* → pods crash-loop or throw 5xx; the fix is often to **restart** the
    deployment (roll the pods) or roll back.
  - *A single wedged pod* → delete just that pod; the Deployment recreates it.

Investigate before acting: `get_pods`, `get_events`, and `describe_pod` show what's actually wrong.
Reach for a mutation (`scale`, `restart`, `delete`) only once the read tools explain the symptom.
