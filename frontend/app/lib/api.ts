// Typed client for the StoaGraph gating-proxy API (github.com/scanset/StAG `serve`
// package). The backend sends permissive CORS, so the browser calls it directly;
// point NEXT_PUBLIC_API_BASE at the stag-serve address (default localhost:8080).

export type Verdict = "allow" | "deny" | "escalate";

export type Chain = {
  sense: string;
  reason: string;
  decide: string;
  act: string;
  prove: string;
};

export type EventView = {
  field: string;
  rule: string;
  actor: string;
  subject: string;
};

export type DecisionView = {
  tool: string;
  verdict: Verdict;
  forward: boolean;
  value: string;
  ruleFired?: string;
  subjectClass: string;
  chain: Chain;
  events?: EventView[];
  fault?: string;
};

export type PolicyView = { tool: string; recipe: string; gateArg: string };

export type VerifyView = {
  count: number;
  head: string;
  signed: boolean;
  keyId?: string;
  verified: boolean;
  error?: string;
};

// One leaf of the audit chain: a decision the gate made. EVERY decision is recorded — allow, deny and
// escalate alike — because a blocked attempt is the evidence the control worked. `releases` are the
// crossings that ACTUALLY happened, so they appear only when `forwarded` is true.
export type RecordView = {
  tool: string;
  verdict: string; // allow | deny | escalate
  forwarded: boolean;
  value: string;
  recipe?: string;
  fault?: string;
  releases?: EventView[];
};
export type LogView = { records: RecordView[]; verify: VerifyView };

const API = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080";

/* ---------------------- control-plane auth (Planning/31) ----------------------
 * stag's control plane requires a bearer token per ROLE. The console is the HUMAN's
 * tool, so it carries `admin` (policy CRUD) and/or `approve` (mints the signed release).
 * It must NEVER be given the orchestrator's `dispatch` token, and the orchestrator must
 * never be given `approve` — that separation is what keeps a hijacked orchestrator from
 * approving its own escalations.
 *
 * The token is held client-side (localStorage): the operator pastes it once. Real user
 * identity (OIDC) is v2 — this is a shared secret for an operator tool, not an identity.
 */
const TOKEN_KEY = "stag.control.token";

export function getToken(): string {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(TOKEN_KEY) ?? "";
}

export function setToken(token: string): void {
  if (typeof window === "undefined") return;
  if (token) window.localStorage.setItem(TOKEN_KEY, token);
  else window.localStorage.removeItem(TOKEN_KEY);
}

/** authFetch attaches the control-plane bearer to every stag-serve call and turns a 401
 *  into an actionable message instead of a confusing parse error. */
async function authFetch(url: string, init?: RequestInit): Promise<Response> {
  const token = getToken();
  const headers = new Headers(init?.headers);
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(url, { ...init, headers });
  if (res.status === 401) {
    throw new Error(
      "401 unauthorized — paste your stag control-plane token (admin/approve) in the sidebar. " +
        "It is generated at deploy/mcp/control.tokens on first start of stag-serve.",
    );
  }
  return res;
}

export async function decide(
  tool: string,
  args: Record<string, string>,
): Promise<DecisionView> {
  const res = await authFetch(`${API}/api/decide`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ tool, args }),
  });
  if (!res.ok) throw new Error(`decide failed (${res.status}): ${await res.text()}`);
  return res.json();
}

export async function getPolicies(): Promise<PolicyView[]> {
  const res = await authFetch(`${API}/api/policies`);
  if (!res.ok) throw new Error(`policies failed (${res.status})`);
  return res.json();
}

export async function getLog(): Promise<LogView> {
  const res = await authFetch(`${API}/api/log`);
  if (!res.ok) throw new Error(`log failed (${res.status})`);
  return res.json();
}

/* -------------------------- recipe authoring -------------------------- */

export type TierRow = { label: string; verdict: string; tier: string };

export type ValidateResult = {
  valid: boolean;
  name: string;
  hash?: string;
  error?: string;
  warnings?: string[];
  tiers?: TierRow[];
};

export type RecipeDetail = { name: string; src: string; result: ValidateResult };

export async function validateRecipe(src: string): Promise<ValidateResult> {
  const res = await authFetch(`${API}/api/recipes/validate`, {
    method: "POST",
    headers: { "Content-Type": "text/plain" },
    body: src,
  });
  return res.json();
}

export async function listRecipes(): Promise<ValidateResult[]> {
  const res = await authFetch(`${API}/api/recipes`);
  if (!res.ok) throw new Error(`list recipes failed (${res.status})`);
  return res.json();
}

export async function getRecipe(name: string): Promise<RecipeDetail> {
  const res = await authFetch(`${API}/api/recipes/${encodeURIComponent(name)}`);
  if (!res.ok) throw new Error(`get recipe failed (${res.status})`);
  return res.json();
}

// saveRecipe returns { ok, result }: on an invalid recipe the backend returns 400
// with the ValidateResult (error), which the editor shows — not thrown.
export async function saveRecipe(src: string): Promise<{ ok: boolean; result: ValidateResult }> {
  const res = await authFetch(`${API}/api/recipes`, {
    method: "POST",
    headers: { "Content-Type": "text/plain" },
    body: src,
  });
  return { ok: res.ok, result: await res.json() };
}

export async function deleteRecipe(name: string): Promise<void> {
  const res = await authFetch(`${API}/api/recipes/${encodeURIComponent(name)}`, { method: "DELETE" });
  if (!res.ok) throw new Error(`delete recipe failed (${res.status})`);
}

/* ------------------------------ adapters ------------------------------ */

export type MCPToolView = { name: string; inputSchema?: string };
export type MCPServerView = {
  name: string;
  transport: string;
  target: string;
  enabled: boolean;
  tools: MCPToolView[];
  discoverError?: string;
  // downstream auth (the raw secret is never returned — only whether one is set + a masked hint)
  authScheme?: string;
  authHeader?: string;
  secretEnv?: string;
  secretSet?: boolean;
  secretHint?: string;
};
export type MCPServerInput = {
  name: string;
  transport: string;
  target: string;
  authScheme?: string;
  authHeader?: string;
  secret?: string; // empty on edit preserves the stored secret
  secretEnv?: string;
  oauthConfig?: string; // optional JSON {client_id, scopes} for providers without dynamic registration
};
export type OAuthStatus = { authorized: boolean; expiresAt?: string; hasRefresh?: boolean };
export type ProviderView = { name: string; kind: string; config: string; enabled: boolean };
// A route DELEGATES: it names the server that serves the tool. The gate never infers the server from
// the tool name — another server exposing the same name must not be able to change an existing route.
export type RouteView = { tool: string; server: string; recipe: string; gateArg: string; valid: boolean; error?: string };

async function jget<T>(path: string): Promise<T> {
  const res = await authFetch(`${API}${path}`);
  if (!res.ok) throw new Error(`${path} failed (${res.status})`);
  return res.json();
}
async function jpost<T>(path: string, body: unknown): Promise<{ ok: boolean; data: T }> {
  const res = await authFetch(`${API}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  return { ok: res.ok, data: await res.json() };
}
async function jdelete(path: string): Promise<void> {
  const res = await authFetch(`${API}${path}`, { method: "DELETE" });
  if (!res.ok) throw new Error(`${path} failed (${res.status})`);
}

// MCP servers (act channel)
export const listMCPServers = () => jget<MCPServerView[]>("/api/mcp-servers");
export const addMCPServer = (s: MCPServerInput) => jpost<MCPServerView>("/api/mcp-servers", s);
export const deleteMCPServer = (name: string) => jdelete(`/api/mcp-servers/${encodeURIComponent(name)}`);

// OAuth sign-in for oauth-scheme downstreams. start returns the provider URL to open; status reports
// whether the gate now holds a usable token. The gate holds the tokens — the browser only relays the code.
export const oauthStart = async (server: string) =>
  (await jpost<{ authUrl: string }>(`/api/oauth/start?server=${encodeURIComponent(server)}`, {})).data;
export const oauthStatus = (server: string) =>
  jget<OAuthStatus>(`/api/oauth/status?server=${encodeURIComponent(server)}`);

// context providers (read channel)
export const listProviders = () => jget<ProviderView[]>("/api/providers");
export const addProvider = (p: { name: string; kind: string; config: string }) =>
  jpost<ProviderView>("/api/providers", p);
export const deleteProvider = (name: string) => jdelete(`/api/providers/${encodeURIComponent(name)}`);

// routes (tool -> recipe bindings)
export const listRoutes = () => jget<RouteView[]>("/api/routes");
export const addRoute = (r: { tool: string; server: string; recipe: string; gateArg: string }) =>
  jpost<{ tool: string }>("/api/routes", r);
// A route is keyed by (server, tool), not by the tool alone: the same tool name may be routed on two
// servers, so deleting `search_code` on `github` must leave `search_code` on `local` untouched.
export const deleteRoute = (server: string, tool: string) =>
  jdelete(`/api/routes/${encodeURIComponent(server)}/${encodeURIComponent(tool)}`);

/* ------------------------------ approvals (Stage 5) ------------------------------ */

export type ApprovalView = {
  id: string;
  tool: string;
  args: Record<string, string>;
  fingerprint: string;
  recipe?: string;
  status: string; // pending | approved | denied | consumed
  tokenIssued: boolean;
  reason?: string;
  createdAt?: string;
  decidedAt?: string;
};

// the human-approval queue: an escalated action awaits a signed release; approve mints it.
export const listApprovals = (status?: string) =>
  jget<ApprovalView[]>(`/api/approvals${status ? `?status=${encodeURIComponent(status)}` : ""}`);
export const approveApproval = (id: string, reason?: string) =>
  jpost<{ id: string; status: string; token: string; keyId: string }>(
    `/api/approvals/${encodeURIComponent(id)}/approve`,
    { reason: reason ?? "" },
  );
export const denyApproval = (id: string, reason?: string) =>
  jpost<{ id: string; status: string }>(`/api/approvals/${encodeURIComponent(id)}/deny`, {
    reason: reason ?? "",
  });


