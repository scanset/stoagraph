// Typed client for the ORCHESTRATOR (harness-serve) — the second backend this console talks to.
//
// Two backends, deliberately NOT merged (Planning/26):
//   stag-serve  :8080  the GATE — policy, approvals, records. Holds no model, no keys.
//   harness-serve :8090  the ORCHESTRATOR — models (API KEYS), the event map, dispatch.
// Only the UI unifies. Keeping the backends separate is what lets the gate stay independently
// runnable, which is the product's whole claim.
//
// They also take DIFFERENT control-plane tokens (Planning/31): `admin`/`approve` for the gate,
// `operator` here. The orchestrator itself only ever holds `dispatch` — it can never approve.

const HARNESS =
  process.env.NEXT_PUBLIC_HARNESS_API || "http://localhost:8090";

const OPERATOR_KEY = "stoagraph.operator.token";

export function getOperatorToken(): string {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(OPERATOR_KEY) ?? "";
}

export function setOperatorToken(token: string): void {
  if (typeof window === "undefined") return;
  if (token) window.localStorage.setItem(OPERATOR_KEY, token);
  else window.localStorage.removeItem(OPERATOR_KEY);
}

async function hfetch(path: string, init?: RequestInit): Promise<Response> {
  const headers = new Headers(init?.headers);
  const token = getOperatorToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  const res = await fetch(`${HARNESS}${path}`, { ...init, headers });
  if (res.status === 401) {
    throw new Error(
      "401 unauthorized — paste the `operator` token in the sidebar (data/control.tokens).",
    );
  }
  return res;
}

/* ------------------------------- models ------------------------------- */
// The orchestrator holds the provider API keys. The raw key NEVER comes back over the wire —
// only keyPresent + a masked hint.

export type ModelView = {
  name: string;
  kind: "claude" | "openai";
  baseUrl?: string;
  model: string;
  apiKeyEnv?: string;
  keyPresent: boolean;
  keyHint?: string;
};

export type ModelInput = {
  name: string;
  kind: string;
  model: string;
  baseUrl?: string;
  apiKey?: string;
  apiKeyEnv?: string;
};

export async function listModels(): Promise<ModelView[]> {
  const res = await hfetch("/api/models");
  if (!res.ok) throw new Error(`models failed (${res.status})`);
  return res.json();
}

export async function saveModel(m: ModelInput): Promise<{ ok: boolean; data: unknown }> {
  const res = await hfetch("/api/models", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(m),
  });
  return { ok: res.ok, data: await res.json() };
}

export async function deleteModel(name: string): Promise<void> {
  const res = await hfetch(`/api/models/${encodeURIComponent(name)}`, { method: "DELETE" });
  if (!res.ok) throw new Error(`delete model failed (${res.status})`);
}

/* ------------------------------ event map ------------------------------ */
// The user-authored deterministic layer: event -> recipe (+ optional toolset and context providers).

export type Definition = {
  id: string;
  match: Record<string, string>;
  recipe: string;
  route?: string;
  tools?: string[];
  context?: string[];
};

export async function getEventMap(): Promise<Definition[]> {
  const res = await hfetch("/api/event-map");
  if (!res.ok) throw new Error(`event map failed (${res.status})`);
  return res.json();
}

export async function saveEventMap(defs: Definition[]): Promise<{ ok: boolean; count?: number; error?: string }> {
  const res = await hfetch("/api/event-map", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(defs),
  });
  return { ok: res.ok, ...(await res.json()) };
}

/* ------------------------------- dispatch ------------------------------ */
// The turnkey path, streamed as SSE: event -> recipe -> session -> governed agent.

export type AgentEvent = {
  kind: string; // dispatch | text | propose | verdict | await | retry | error | done
  tool?: string;
  text?: string;
  result?: string;
  allowed?: boolean;
};

export type DispatchRequest = {
  event: unknown;
  model: string;
  dispatchModel?: string;
  system?: string;
  maxTurns?: number;
};

/** dispatch streams the governed run. onEvent fires per SSE frame; the promise resolves when the
 *  stream ends. Abort with the returned controller to stop mid-run. */
export function dispatch(
  req: DispatchRequest,
  onEvent: (e: AgentEvent) => void,
): { done: Promise<void>; abort: () => void } {
  const ctl = new AbortController();
  const done = (async () => {
    const res = await hfetch("/api/dispatch", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(req),
      signal: ctl.signal,
    });
    if (!res.ok || !res.body) throw new Error(`dispatch failed (${res.status})`);

    const reader = res.body.getReader();
    const dec = new TextDecoder();
    let buf = "";
    for (;;) {
      const { done: fin, value } = await reader.read();
      if (fin) break;
      buf += dec.decode(value, { stream: true });
      // SSE frames are separated by a blank line; each carries one `data:` line.
      const frames = buf.split("\n\n");
      buf = frames.pop() ?? "";
      for (const frame of frames) {
        const line = frame.split("\n").find((l) => l.startsWith("data:"));
        if (!line) continue;
        try {
          onEvent(JSON.parse(line.slice(5).trim()) as AgentEvent);
        } catch {
          /* a partial/garbled frame is skipped, never fatal */
        }
      }
    }
  })();
  return { done, abort: () => ctl.abort() };
}
