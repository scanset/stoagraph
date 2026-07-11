"use client";

import { useCallback, useEffect, useState } from "react";
import {
  deleteModel,
  listModels,
  saveModel,
  type ModelView,
} from "../lib/harness";

/* Models live in the ORCHESTRATOR, never in the gate. This page is the one place API keys are
 * entered, and the raw key never comes back over the wire — the backend returns only keyPresent
 * plus a masked hint. That asymmetry is the point: the gate can be handed to anyone precisely
 * because it holds no keys and no model. */
export default function ModelsPage() {
  const [models, setModels] = useState<ModelView[]>([]);
  const [msg, setMsg] = useState<string | null>(null);

  const refresh = useCallback(() => {
    listModels()
      .then((m) => {
        setModels(m);
        setMsg(null);
      })
      .catch((e) => setMsg(e instanceof Error ? e.message : String(e)));
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
          <h1 className="text-[15px] font-semibold tracking-tight">Models</h1>
          <span className="text-sm text-[var(--faint)]">the proposer + dispatcher — keys live here, never in the gate</span>
        </div>
        {msg && <span className="text-[12px] text-[var(--deny)]">⚠ {msg}</span>}
      </header>

      <div className="flex-1 overflow-auto p-6">
        <div className="mx-auto flex max-w-3xl flex-col gap-6">
          <Card title="Connected" sub="the untrusted proposer — the gate decides what it may do">
            <div className="flex flex-col gap-2 p-4">
              {models.length === 0 && (
                <div className="py-6 text-center text-[12px] text-[var(--faint)]">
                  No models connected.
                </div>
              )}
              {models.map((m) => (
                <div
                  key={m.name}
                  className="flex items-center justify-between gap-3 rounded-lg border border-[var(--border)] bg-[var(--panel-2)] px-3 py-2.5"
                >
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-[13px]">{m.name}</span>
                      <span className="rounded bg-[var(--accent-soft)] px-1.5 py-0.5 text-[10px] text-[var(--muted)]">
                        {m.kind}
                      </span>
                      <span className="truncate font-mono text-[11px] text-[var(--muted)]">{m.model}</span>
                    </div>
                    <div className="mt-1 text-[11px] text-[var(--faint)]">
                      {m.keyHint ? `key ${m.keyHint}` : `env ${m.apiKeyEnv || "—"}`}{" "}
                      <span style={{ color: m.keyPresent ? "var(--allow)" : "var(--deny)" }}>
                        ● {m.keyPresent ? "present" : "missing"}
                      </span>
                    </div>
                  </div>
                  <button
                    onClick={() => wrap(async () => deleteModel(m.name))}
                    className="text-[var(--faint)] hover:text-[var(--deny)]"
                    aria-label="delete"
                  >
                    ✕
                  </button>
                </div>
              ))}
            </div>
          </Card>

          <AddModel wrap={wrap} />
        </div>
      </div>
    </>
  );
}

function AddModel({ wrap }: { wrap: (fn: () => Promise<void>) => void }) {
  const [name, setName] = useState("");
  const [kind, setKind] = useState("claude");
  const [model, setModel] = useState("");
  const [baseUrl, setBaseUrl] = useState("");
  const [apiKey, setApiKey] = useState("");
  const [apiKeyEnv, setApiKeyEnv] = useState("");

  const submit = () =>
    wrap(async () => {
      const res = await saveModel({ name, kind, model, baseUrl, apiKey, apiKeyEnv });
      if (!res.ok) throw new Error((res.data as { error?: string })?.error ?? "save failed");
      setName("");
      setModel("");
      setBaseUrl("");
      setApiKey("");
      setApiKeyEnv("");
    });

  return (
    <Card title="Connect a model" sub="openai-compatible needs a base URL">
      <div className="flex flex-col gap-3 p-4">
        <div className="flex gap-2">
          <Field label="name">
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="claude" />
          </Field>
          <Field label="dialect">
            <select
              value={kind}
              onChange={(e) => setKind(e.target.value)}
              className="min-w-0 flex-1 rounded-md border border-[var(--border-strong)] bg-[var(--panel-2)] px-2.5 py-1.5 font-mono text-[12px] text-[var(--text)] outline-none focus:border-[var(--accent)]"
            >
              <option value="claude">claude</option>
              <option value="openai">openai-compatible</option>
            </select>
          </Field>
        </div>
        <Field label="model id">
          <Input value={model} onChange={(e) => setModel(e.target.value)} placeholder="claude-opus-4-8" />
        </Field>
        {kind === "openai" && (
          <Field label="base URL">
            <Input
              value={baseUrl}
              onChange={(e) => setBaseUrl(e.target.value)}
              placeholder="https://openrouter.ai/api/v1"
            />
          </Field>
        )}
        <div className="flex gap-2">
          <Field label="API key (stored)">
            <Input
              type="password"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder="sk-…"
              autoComplete="off"
            />
          </Field>
          <Field label="…or key env var">
            <Input
              value={apiKeyEnv}
              onChange={(e) => setApiKeyEnv(e.target.value)}
              placeholder="ANTHROPIC_API_KEY"
            />
          </Field>
        </div>
        <div>
          <button
            onClick={submit}
            disabled={!name || !model}
            className="rounded-md bg-[var(--accent)] px-3.5 py-1.5 text-[12px] font-medium text-[#04122b] disabled:opacity-40"
          >
            Connect
          </button>
        </div>
      </div>
    </Card>
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

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="flex min-w-0 flex-1 flex-col gap-1 text-[11px] font-medium text-[var(--muted)]">
      {label}
      <div className="flex">{children}</div>
    </label>
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
