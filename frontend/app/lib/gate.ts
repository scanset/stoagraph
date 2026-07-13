// Small gate-state helpers for the console shell (no demo wiring).

import { listMCPServers, listRoutes } from "./api";

/** isGateEmpty reports whether nothing has been authored yet — the trigger for onboarding. A fresh
 *  gate has no recipes, no routes, no servers. (Cheap: two list calls; recipes are implied by routes.) */
export async function isGateEmpty(): Promise<boolean> {
  const [routes, servers] = await Promise.all([listRoutes(), listMCPServers()]);
  return routes.length === 0 && servers.length === 0;
}
