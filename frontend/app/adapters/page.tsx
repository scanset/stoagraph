"use client";

import { useCallback, useEffect, useState } from "react";
import {
  addMCPServer,
  addProvider,
  addRoute,
  deleteMCPServer,
  deleteProvider,
  deleteRoute,
  listMCPServers,
  listProviders,
  listRecipes,
  listRoutes,
  type MCPServerView,
  type ProviderView,
  type RouteView,
  type ValidateResult,
} from "../lib/api";

export default function AdaptersPage() {
  const [servers, setServers] = useState<MCPServerView[]>([]);
  const [providers, setProviders] = useState<ProviderView[]>([]);
  const [routes, setRoutes] = useState<RouteView[]>([]);
  const [recipes, setRecipes] = useState<ValidateResult[]>([]);
  const [msg, setMsg] = useState<string | null>(null);

  const refresh = useCallback(() => {
    listMCPServers().then(setServers).catch(() => {});
    listProviders().then(setProviders).catch(() => {});
    listRoutes().then(setRoutes).catch(() => {});
    listRecipes().then(setRecipes).catch(() => {});
  }, []);
  useEffect(() => refresh(), [refresh]);

  const wrap = useCallback(
    async (fn: () => Promise<void>) => {
      setMsg(null);
      try {
        await fn();
        refresh();
      } catch (e) {
        setMsg(e instanceof Error ? e.message : String(e));
      }
    },
    [refresh],
  );

  return (
    <>
      <header className="flex h-14 shrink-0 items-center justify-between border-b border-[var(--border)] px-6">
        <div className="flex items-baseline gap-3">
          <h1 className="text-[15px] font-semibold tracking-tight">Adapters</h1>
          <span className="text-sm text-[var(--faint)]">the boundary — everything the agent reads and does</span>
        </div>
        {msg && <span className="text-[12px] text-[var(--deny)]">⚠ {msg}</span>}
      </header>

      <div className="flex-1 overflow-auto p-6">
        <div className="flex flex-col gap-6">
          <MCPServers servers={servers} wrap={wrap} />
          <div className="grid grid-cols-1 gap-6 xl:grid-cols-2">
            <Providers providers={providers} wrap={wrap} />
            <Routes routes={routes} recipes={recipes} servers={servers} wrap={wrap} />
          </div>
        </div>
      </div>
    </>
  );
}

/* ------------------------------ shared ------------------------------ */

function Card({ title, sub, children }: { title: string; sub: string; children: React.ReactNode }) {
  return (
    <div className="overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--panel)]">
      <div className="flex items-baseline gap-2.5 border-b border-[var(--border)] px-5 py-3.5">
        <span className="text-sm font-semibold">{title}</span>
        <span className="text-xs text-[var(--faint)]">{sub}</span>
      </div>
      {children}
    </div>
  );
}

function Input(props: React.InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      {...props}
      className="min-w-0 flex-1 rounded-md border border-[var(--border-strong)] bg-[var(--panel-2)] px-2.5 py-1.5 font-mono text-[12px] text-[var(--text)] outline-none focus:border-[var(--accent)]"
    />
  );
}

function AddBtn({ onClick, disabled }: { onClick: () => void; disabled?: boolean }) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className="rounded-md bg-[var(--accent)] px-3.5 py-1.5 text-[12px] font-medium text-[#04122b] disabled:opacity-40"
    >
      Add
    </button>
  );
}

function Del({ onClick }: { onClick: () => void }) {
  return (
    <button onClick={onClick} className="text-[var(--faint)] hover:text-[var(--deny)]" aria-label="delete">
      ✕
    </button>
  );
}

/* --------------------------- mcp servers --------------------------- */

function MCPServers({ servers, wrap }: { servers: MCPServerView[]; wrap: (fn: () => Promise<void>) => void }) {
  const [name, setName] = useState("");
  const [transport, setTransport] = useState("stdio");
  const [target, setTarget] = useState("");
  const [authScheme, setAuthScheme] = useState("none");
  const [authHeader, setAuthHeader] = useState("");
  const [secret, setSecret] = useState("");
  const [secretEnv, setSecretEnv] = useState("");

  const selectCls = "rounded-md border border-[var(--border-strong)] bg-[var(--panel-2)] px-2 py-1.5 text-[12px] text-[var(--text)]";
  const showAuth = transport === "http" && authScheme !== "none";

  return (
    <Card title="MCP servers" sub="act channel · tool calls gated before they reach these">
      <div className="flex flex-wrap items-center gap-2 border-b border-[var(--border)] px-5 py-3">
        <Input placeholder="name" value={name} onChange={(e) => setName(e.target.value)} />
        <select value={transport} onChange={(e) => setTransport(e.target.value)} className={selectCls}>
          <option value="stdio">stdio</option>
          <option value="http">http</option>
        </select>
        <Input
          placeholder={transport === "stdio" ? "command (e.g. npx server-filesystem)" : "https://host/mcp"}
          value={target}
          onChange={(e) => setTarget(e.target.value)}
        />
        {transport === "http" && (
          <select value={authScheme} onChange={(e) => setAuthScheme(e.target.value)} className={selectCls} title="downstream auth (the gate holds the credential)">
            <option value="none">no auth</option>
            <option value="bearer">bearer</option>
            <option value="header">header</option>
          </select>
        )}
        {showAuth && authScheme === "header" && (
          <Input placeholder="header name (e.g. X-API-Key)" value={authHeader} onChange={(e) => setAuthHeader(e.target.value)} />
        )}
        {showAuth && (
          <>
            <Input placeholder="secret env var (preferred)" value={secretEnv} onChange={(e) => setSecretEnv(e.target.value)} />
            <input
              type="password"
              placeholder="or paste secret (dev)"
              value={secret}
              onChange={(e) => setSecret(e.target.value)}
              className="min-w-[10rem] flex-1 rounded-md border border-[var(--border-strong)] bg-[var(--panel-2)] px-2.5 py-1.5 text-[12px] text-[var(--text)] outline-none placeholder:text-[var(--faint)]"
            />
          </>
        )}
        <AddBtn
          disabled={!name || !target || (showAuth && authScheme === "header" && !authHeader) || (showAuth && !secret && !secretEnv)}
          onClick={() =>
            wrap(async () => {
              await addMCPServer({ name, transport, target, authScheme, authHeader, secret, secretEnv });
              setName("");
              setTarget("");
              setSecret("");
              setSecretEnv("");
              setAuthHeader("");
              setAuthScheme("none");
            })
          }
        />
      </div>
      {servers.length === 0 ? (
        <Empty>No MCP servers connected. Add one to discover its tools.</Empty>
      ) : (
        servers.map((s) => (
          <div key={s.name} className="flex items-start justify-between border-t border-[var(--border)] px-5 py-3 first:border-t-0">
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <span className="font-mono text-[13px] text-[var(--text)]">{s.name}</span>
                <span className="rounded bg-[var(--panel-3)] px-1.5 py-0.5 text-[10px] text-[var(--muted)]">{s.transport}</span>
                {s.authScheme && s.authScheme !== "none" && (
                  <span className="rounded bg-[var(--panel-3)] px-1.5 py-0.5 text-[10px] text-[var(--accent)]" title={s.secretEnv ? `env ${s.secretEnv}` : s.secretHint ? `key ${s.secretHint}` : "no secret set"}>
                    🔒 {s.authScheme}{s.secretSet || s.secretEnv ? "" : " ⚠"}
                  </span>
                )}
                <span className="truncate font-mono text-[11px] text-[var(--faint)]">{s.target}</span>
              </div>
              {s.discoverError ? (
                <div className="mt-1 text-[11px] text-[var(--deny)]">unreachable: {s.discoverError}</div>
              ) : (
                <div className="mt-1 flex flex-wrap gap-1">
                  {s.tools.length === 0 && <span className="text-[11px] text-[var(--faint)]">no tools</span>}
                  {s.tools.map((t) => (
                    <span key={t.name} className="rounded border border-[var(--border)] px-1.5 py-0.5 font-mono text-[10px] text-[var(--allow)]">
                      {t.name}
                    </span>
                  ))}
                </div>
              )}
            </div>
            <Del onClick={() => wrap(() => deleteMCPServer(s.name))} />
          </div>
        ))
      )}
    </Card>
  );
}

/* --------------------------- providers --------------------------- */

function Providers({ providers, wrap }: { providers: ProviderView[]; wrap: (fn: () => Promise<void>) => void }) {
  const [name, setName] = useState("");
  const [kind, setKind] = useState("http");
  const [config, setConfig] = useState("");

  return (
    <Card title="Context providers" sub="read channel · everything they return is untrusted + recorded">
      <div className="flex flex-wrap items-center gap-2 border-b border-[var(--border)] px-5 py-3">
        <Input placeholder="name" value={name} onChange={(e) => setName(e.target.value)} />
        <select
          value={kind}
          onChange={(e) => setKind(e.target.value)}
          className="rounded-md border border-[var(--border-strong)] bg-[var(--panel-2)] px-2 py-1.5 text-[12px] text-[var(--text)]"
        >
          <option value="http">http</option>
          <option value="rag">rag</option>
          <option value="mcp_resource">mcp_resource</option>
        </select>
        <Input placeholder="config (url / dir / json)" value={config} onChange={(e) => setConfig(e.target.value)} />
        <AddBtn
          disabled={!name}
          onClick={() =>
            wrap(async () => {
              await addProvider({ name, kind, config });
              setName("");
              setConfig("");
            })
          }
        />
      </div>
      {providers.length === 0 ? (
        <Empty>No context providers.</Empty>
      ) : (
        providers.map((p) => (
          <div key={p.name} className="flex items-center justify-between border-t border-[var(--border)] px-5 py-2.5 first:border-t-0">
            <div className="flex min-w-0 items-center gap-2">
              <span className="font-mono text-[13px] text-[var(--text)]">{p.name}</span>
              <span className="rounded bg-[var(--panel-3)] px-1.5 py-0.5 text-[10px] text-[var(--muted)]">{p.kind}</span>
              <span className="truncate font-mono text-[11px] text-[var(--faint)]">{p.config}</span>
            </div>
            <Del onClick={() => wrap(() => deleteProvider(p.name))} />
          </div>
        ))
      )}
    </Card>
  );
}

/* --------------------------- routes --------------------------- */

function Routes({
  routes,
  recipes,
  servers,
  wrap,
}: {
  routes: RouteView[];
  recipes: ValidateResult[];
  servers: MCPServerView[];
  wrap: (fn: () => Promise<void>) => void;
}) {
  const [tool, setTool] = useState("");
  const [recipe, setRecipe] = useState("");
  const [gateArg, setGateArg] = useState("");
  const tools = servers.flatMap((s) => s.tools.map((t) => t.name));

  return (
    <Card title="Route bindings" sub="tool → recipe · the gate's source of truth">
      <div className="flex flex-wrap items-center gap-2 border-b border-[var(--border)] px-5 py-3">
        <Input placeholder="tool" value={tool} onChange={(e) => setTool(e.target.value)} list="tools" />
        <datalist id="tools">
          {tools.map((t) => (
            <option key={t} value={t} />
          ))}
        </datalist>
        <select
          value={recipe}
          onChange={(e) => setRecipe(e.target.value)}
          className="rounded-md border border-[var(--border-strong)] bg-[var(--panel-2)] px-2 py-1.5 text-[12px] text-[var(--text)]"
        >
          <option value="">recipe…</option>
          {recipes.filter((r) => r.valid).map((r) => (
            <option key={r.name} value={r.name}>
              {r.name}
            </option>
          ))}
        </select>
        <Input placeholder="arg" value={gateArg} onChange={(e) => setGateArg(e.target.value)} />
        <AddBtn
          disabled={!tool || !recipe || !gateArg}
          onClick={() =>
            wrap(async () => {
              await addRoute({ tool, recipe, gateArg });
              setTool("");
              setGateArg("");
            })
          }
        />
      </div>
      {routes.length === 0 ? (
        <Empty>No routes. Bind a tool to a recipe to gate it.</Empty>
      ) : (
        routes.map((r) => (
          <div key={r.tool} className="flex items-center justify-between border-t border-[var(--border)] px-5 py-2.5 first:border-t-0">
            <div className="flex min-w-0 items-center gap-2">
              <span className="h-1.5 w-1.5 shrink-0 rounded-full" style={{ background: r.valid ? "var(--allow)" : "var(--deny)" }} />
              <span className="font-mono text-[13px] text-[var(--text)]">{r.tool}</span>
              <span className="text-[var(--faint)]">→</span>
              <span className="font-mono text-[12px] text-[var(--muted)]">{r.recipe}</span>
              <span className="font-mono text-[11px] text-[var(--faint)]">({r.gateArg})</span>
              {!r.valid && <span className="text-[11px] text-[var(--deny)]">unresolved</span>}
            </div>
            <Del onClick={() => wrap(() => deleteRoute(r.tool))} />
          </div>
        ))
      )}
    </Card>
  );
}

function Empty({ children }: { children: React.ReactNode }) {
  return <div className="px-5 py-8 text-center text-sm text-[var(--faint)]">{children}</div>;
}
