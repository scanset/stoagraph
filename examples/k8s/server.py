#!/usr/bin/env python3
"""k8s-ops MCP server (stdio, zero-dependency) — REAL infra, read-only v1.

Same StoaGraph architecture as the mock demos (pii-demo, zt-ops) pointed at a real
Kubernetes cluster: this server exposes cluster operations as MCP tools by shelling out to
the LOCAL `kubectl` (inheriting your kubeconfig). v1 is READ-ONLY — safe to run against a
live cluster to prove the wiring; mutating/destructive tools get added (and gated) as the
test case pushes.

Safety: every tool runs kubectl via subprocess with a LIST of args (never a shell string),
so a namespace like "; rm -rf" is an inert literal, not a shell exec. The real control is
still StoaGraph's gate in front of this server — this server just executes what it's given.
"""

import json
import subprocess
import sys


def kubectl(args, timeout=20):
    """Run `kubectl <args>` (list form; no shell). Returns combined output text."""
    try:
        p = subprocess.run(["kubectl", *args], capture_output=True, text=True, timeout=timeout)
    except FileNotFoundError:
        return "error: kubectl not found on PATH"
    except subprocess.TimeoutExpired:
        return "error: kubectl timed out"
    out = (p.stdout or "") + (p.stderr or "")
    return out.strip()[:8000] or "(no output)"


def obj(props, required):
    return {"type": "object", "properties": props, "required": required}


NS = {"namespace": {"type": "string", "description": "kubernetes namespace"}}

TOOLS = [
    {"name": "get_pods", "description": "List pods in a namespace.",
     "inputSchema": obj(NS, ["namespace"]),
     "run": lambda a: kubectl(["get", "pods", "-n", a.get("namespace", "default"), "-o", "wide"])},
    {"name": "get_deployments", "description": "List deployments in a namespace.",
     "inputSchema": obj(NS, ["namespace"]),
     "run": lambda a: kubectl(["get", "deployments", "-n", a.get("namespace", "default"), "-o", "wide"])},
    {"name": "get_pod_logs", "description": "Tail logs from a pod.",
     "inputSchema": obj({**NS, "pod": {"type": "string"}, "tail": {"type": "string", "description": "lines, default 50"}}, ["namespace", "pod"]),
     "run": lambda a: kubectl(["logs", a.get("pod", ""), "-n", a.get("namespace", "default"), "--tail", str(a.get("tail", "50"))])},
    {"name": "describe_pod", "description": "Describe a pod (status, events, conditions).",
     "inputSchema": obj({**NS, "pod": {"type": "string"}}, ["namespace", "pod"]),
     "run": lambda a: kubectl(["describe", "pod", a.get("pod", ""), "-n", a.get("namespace", "default")])},
    {"name": "get_nodes", "description": "List cluster nodes.",
     "inputSchema": obj({}, []),
     "run": lambda a: kubectl(["get", "nodes", "-o", "wide"])},
    {"name": "get_events", "description": "Recent events in a namespace.",
     "inputSchema": obj(NS, ["namespace"]),
     "run": lambda a: kubectl(["get", "events", "-n", a.get("namespace", "default"), "--sort-by", ".lastTimestamp"])},

    # --- MUTATING tools (gated: scale graded by count, restart/delete tiered by namespace,
    #     delete_namespace hard-denied). An ALLOWED call really changes the cluster. ---
    {"name": "scale_deployment", "description": "Scale a deployment to N replicas.",
     "inputSchema": obj({**NS, "deployment": {"type": "string"}, "replicas": {"type": "string", "description": "replica count"}}, ["namespace", "deployment", "replicas"]),
     "run": lambda a: kubectl(["scale", "deployment", a.get("deployment", ""), "-n", a.get("namespace", "default"), "--replicas", str(a.get("replicas", "1"))])},
    {"name": "restart_deployment", "description": "Rolling-restart a deployment.",
     "inputSchema": obj({**NS, "deployment": {"type": "string"}}, ["namespace", "deployment"]),
     "run": lambda a: kubectl(["rollout", "restart", "deployment", a.get("deployment", ""), "-n", a.get("namespace", "default")])},
    {"name": "delete_pod", "description": "Delete a pod (its controller recreates it).",
     "inputSchema": obj({**NS, "pod": {"type": "string"}}, ["namespace", "pod"]),
     "run": lambda a: kubectl(["delete", "pod", a.get("pod", ""), "-n", a.get("namespace", "default")])},
    {"name": "delete_deployment", "description": "Delete a deployment.",
     "inputSchema": obj({**NS, "deployment": {"type": "string"}}, ["namespace", "deployment"]),
     "run": lambda a: kubectl(["delete", "deployment", a.get("deployment", ""), "-n", a.get("namespace", "default")])},
    {"name": "delete_namespace", "description": "Permanently delete a namespace and everything in it.",
     "inputSchema": obj(NS, ["namespace"]),
     "run": lambda a: kubectl(["delete", "namespace", a.get("namespace", "")])},
]

BY_NAME = {t["name"]: t for t in TOOLS}
LISTED = [{k: t[k] for k in ("name", "description", "inputSchema")} for t in TOOLS]


def handle(msg):
    mid, method = msg.get("id"), msg.get("method")
    if method == "initialize":
        ver = (msg.get("params") or {}).get("protocolVersion", "2025-06-18")
        return {"jsonrpc": "2.0", "id": mid, "result": {
            "protocolVersion": ver, "capabilities": {"tools": {}},
            "serverInfo": {"name": "k8s-ops", "version": "0.1.0"}}}
    if method == "tools/list":
        return {"jsonrpc": "2.0", "id": mid, "result": {"tools": LISTED}}
    if method == "tools/call":
        p = msg.get("params") or {}
        t = BY_NAME.get(p.get("name"))
        if t is None:
            return {"jsonrpc": "2.0", "id": mid, "result": {"content": [{"type": "text", "text": f"unknown tool {p.get('name')!r}"}], "isError": True}}
        text = t["run"](p.get("arguments") or {})
        return {"jsonrpc": "2.0", "id": mid, "result": {"content": [{"type": "text", "text": text}]}}
    if method == "ping":
        return {"jsonrpc": "2.0", "id": mid, "result": {}}
    if mid is None:
        return None
    return {"jsonrpc": "2.0", "id": mid, "error": {"code": -32601, "message": f"method not found: {method}"}}


def main():
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            msg = json.loads(line)
        except json.JSONDecodeError:
            continue
        resp = handle(msg)
        if resp is not None:
            sys.stdout.write(json.dumps(resp) + "\n")
            sys.stdout.flush()


if __name__ == "__main__":
    main()
