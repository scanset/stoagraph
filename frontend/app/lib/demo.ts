// One-click demo loader for the empty-state onboarding — the in-console equivalent of `stoagraph demo`.
//
// The two recipes are embedded so the button works with no round-trip to disk or GitHub. Source of
// truth: examples/pii-demo/recipes/. They are tiny and frozen; if you change those, change these.

import { addMCPServer, addRoute, listMCPServers, listRoutes, saveRecipe } from "./api";

const INTERNAL_LOOKUP = `recipe: internal_lookup_policy
version: 1
steps:
  - id: propose_uid
    kind: propose
    out: uid
  - id: read
    kind: sink
    in: uid
    field: internal.user_lookup
    sensitivity: benign
`;

const EXTERNAL_REPLY = `recipe: external_reply_policy
version: 1
rules:
  reply.templates:
    kind: set_membership
    set: ["tmpl:account_unlocked", "tmpl:password_reset_sent", "tmpl:ticket_escalated", "tmpl:looking_into_it"]
steps:
  - id: propose_body
    kind: propose
    out: body
  - id: send
    kind: sink
    in: body
    field: outbound.email.body
    sensitivity: authoritative
    rule: reply.templates
    actor: "policy:outbound_comms"
`;

/** loadDemo authors the containment policy, registers the pii-demo tool server, and routes its tools.
 *  It is idempotent enough to re-run: recipes upsert, and a duplicate server/route is harmless. Returns
 *  a short status per step so the UI can show progress. */
export async function loadDemo(): Promise<string[]> {
  const steps: string[] = [];

  await saveRecipe(INTERNAL_LOOKUP);
  await saveRecipe(EXTERNAL_REPLY);
  steps.push("authored 2 policies");

  // The pii-demo tool server ships as a compose service, reachable on the docker network.
  await addMCPServer({ name: "pii-demo", transport: "http", target: "http://pii-demo:9000/mcp" });
  steps.push("registered the pii-demo tool server");

  await addRoute({ tool: "fetch_user_profile", server: "pii-demo", recipe: "internal_lookup_policy", gateArg: "user_id" });
  await addRoute({ tool: "send_external_reply", server: "pii-demo", recipe: "external_reply_policy", gateArg: "message_body" });
  steps.push("routed 2 tools");

  return steps;
}

/** isGateEmpty reports whether nothing has been authored yet — the trigger for onboarding. A fresh
 *  gate has no recipes, no routes, no servers. (Cheap: two list calls; recipes are implied by routes.) */
export async function isGateEmpty(): Promise<boolean> {
  const [routes, servers] = await Promise.all([listRoutes(), listMCPServers()]);
  return routes.length === 0 && servers.length === 0;
}
