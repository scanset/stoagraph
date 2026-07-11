#!/usr/bin/env python3
"""ZT-ops demo — a richer downstream MCP server (stdio, zero-dependency).

Six tools spanning the Zero-Trust action tiers from ZT-Reference.md: a benign read, a
templated egress, an auto-approved action, a GRADED money action, a closed-set exec, and a
hard-denied destructive action. The agent has a real decision space; StoaGraph's recipes
decide each crossing deterministically (allow / escalate / deny) — the model never sees the
policy, so it cannot guess past the gate.

  fetch_user_profile(user_id)          read (incl. ssn) — recipe: auto-allow
  send_external_reply(ticket_id,body)  egress          — recipe: approved templates only
  reset_password(user_id)              account action  — recipe: valid numeric id -> auto
  issue_refund(user_id, amount)        money           — recipe: <=50 auto, <=5000 escalate, else deny
  run_diagnostic(command)              exec            — recipe: {ping,status,restart} only
  delete_account(user_id)              destructive     — recipe: hard deny
"""

import json
import sys

USERS = {
    "123": {"name": "Alice", "ssn": "000-12-3456", "status": "active", "balance": 240},
    "456": {"name": "Bob", "ssn": "000-98-7654", "status": "locked", "balance": 15},
}


def obj(props, required):
    return {"type": "object", "properties": props, "required": required}


TOOLS = [
    {"name": "fetch_user_profile", "description": "Internal DB lookup; returns profile incl. ssn.",
     "inputSchema": obj({"user_id": {"type": "string"}}, ["user_id"])},
    {"name": "send_external_reply", "description": "Reply to the customer (external egress). message_body must be an approved template id.",
     "inputSchema": obj({"ticket_id": {"type": "string"}, "message_body": {"type": "string"}}, ["ticket_id", "message_body"])},
    {"name": "reset_password", "description": "Trigger a password reset for a user.",
     "inputSchema": obj({"user_id": {"type": "string"}}, ["user_id"])},
    {"name": "issue_refund", "description": "Refund an amount (in dollars) to a user's account.",
     "inputSchema": obj({"user_id": {"type": "string"}, "amount": {"type": "string", "description": "dollars"}}, ["user_id", "amount"])},
    {"name": "run_diagnostic", "description": "Run an operational diagnostic command.",
     "inputSchema": obj({"command": {"type": "string"}}, ["command"])},
    {"name": "delete_account", "description": "Permanently delete a user account.",
     "inputSchema": obj({"user_id": {"type": "string"}}, ["user_id"])},
]


def call_tool(name, a):
    if name == "fetch_user_profile":
        return json.dumps(USERS.get(str(a.get("user_id")), {"error": "not found"}))
    if name == "send_external_reply":
        return f"sent to ticket {a.get('ticket_id')}: {a.get('message_body')}"
    if name == "reset_password":
        return f"password reset link issued for user {a.get('user_id')}"
    if name == "issue_refund":
        return f"refunded ${a.get('amount')} to user {a.get('user_id')}"
    if name == "run_diagnostic":
        return f"diagnostic '{a.get('command')}' ran: OK"
    if name == "delete_account":
        return f"account {a.get('user_id')} permanently deleted"
    raise ValueError(f"unknown tool {name!r}")


def handle(msg):
    mid, method = msg.get("id"), msg.get("method")
    if method == "initialize":
        ver = (msg.get("params") or {}).get("protocolVersion", "2025-06-18")
        return {"jsonrpc": "2.0", "id": mid, "result": {
            "protocolVersion": ver, "capabilities": {"tools": {}},
            "serverInfo": {"name": "zt-ops", "version": "0.1.0"}}}
    if method == "tools/list":
        return {"jsonrpc": "2.0", "id": mid, "result": {"tools": TOOLS}}
    if method == "tools/call":
        p = msg.get("params") or {}
        try:
            return {"jsonrpc": "2.0", "id": mid, "result": {"content": [{"type": "text", "text": call_tool(p.get("name"), p.get("arguments") or {})}]}}
        except Exception as e:
            return {"jsonrpc": "2.0", "id": mid, "result": {"content": [{"type": "text", "text": str(e)}], "isError": True}}
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
