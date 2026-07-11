"use client";

import { useCallback, useEffect, useState } from "react";
import {
  deleteRecipe,
  getRecipe,
  listRecipes,
  saveRecipe,
  validateRecipe,
  type TierRow,
  type ValidateResult,
} from "../lib/api";

const TEMPLATE = `recipe: my_policy
version: 1
rules:
  action.allowed:
    kind: set_membership
    set: ["safe_value"]
steps:
  - id: propose
    kind: propose
    out: v
  - id: apply
    kind: sink
    in: v
    field: mcp.tool.arg
    sensitivity: authoritative
    rule: action.allowed
    actor: "policy:example"
`;

const TIER: Record<string, { label: string; color: string; soft: string }> = {
  auto: { label: "AUTO", color: "var(--allow)", soft: "var(--allow-soft)" },
  benign: { label: "BENIGN", color: "var(--muted)", soft: "rgba(255,255,255,0.05)" },
  escalate: { label: "ESCALATE", color: "var(--escalate)", soft: "var(--escalate-soft)" },
  deny: { label: "DENY", color: "var(--deny)", soft: "var(--deny-soft)" },
};

export default function RecipesPage() {
  const [recipes, setRecipes] = useState<ValidateResult[]>([]);
  const [src, setSrc] = useState(TEMPLATE);
  const [result, setResult] = useState<ValidateResult | null>(null);
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const refreshList = useCallback(() => {
    listRecipes().then(setRecipes).catch(() => setRecipes([]));
  }, []);

  useEffect(() => {
    refreshList();
  }, [refreshList]);

  // live validation, debounced
  useEffect(() => {
    const id = setTimeout(() => {
      validateRecipe(src).then(setResult).catch(() => setResult(null));
    }, 350);
    return () => clearTimeout(id);
  }, [src]);

  const load = useCallback(async (name: string) => {
    setMsg(null);
    try {
      const d = await getRecipe(name);
      setSrc(d.src);
      setResult(d.result);
      setSelectedName(name);
    } catch (e) {
      setMsg(e instanceof Error ? e.message : String(e));
    }
  }, []);

  const onNew = useCallback(() => {
    setSrc(TEMPLATE);
    setSelectedName(null);
    setMsg(null);
  }, []);

  const onSave = useCallback(async () => {
    setSaving(true);
    setMsg(null);
    try {
      const { ok, result: r } = await saveRecipe(src);
      setResult(r);
      if (ok) {
        setSelectedName(r.name);
        setMsg(`saved ${r.name}`);
        refreshList();
      } else {
        setMsg(`not saved: ${r.error ?? "invalid recipe"}`);
      }
    } catch (e) {
      setMsg(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }, [src, refreshList]);

  const onDelete = useCallback(async () => {
    if (!selectedName) return;
    try {
      await deleteRecipe(selectedName);
      onNew();
      refreshList();
    } catch (e) {
      setMsg(e instanceof Error ? e.message : String(e));
    }
  }, [selectedName, onNew, refreshList]);

  return (
    <>
      <header className="flex h-14 shrink-0 items-center justify-between border-b border-[var(--border)] px-6">
        <div className="flex items-baseline gap-3">
          <h1 className="text-[15px] font-semibold tracking-tight">Recipes</h1>
          <span className="text-sm text-[var(--faint)]">{recipes.length} policies · validated by the real linter</span>
        </div>
        <button
          onClick={onNew}
          className="rounded-lg border border-[var(--border-strong)] px-3.5 py-1.5 text-sm text-[var(--text)] hover:bg-white/[0.03]"
        >
          + New recipe
        </button>
      </header>

      <div className="flex-1 overflow-hidden p-6">
        <div className="grid h-full grid-cols-[240px_1fr_340px] gap-5">
          {/* list */}
          <div className="overflow-auto rounded-xl border border-[var(--border)] bg-[var(--panel)]">
            <div className="border-b border-[var(--border)] px-4 py-3 text-xs uppercase tracking-wider text-[var(--faint)]">
              Stored
            </div>
            {recipes.length === 0 && <div className="px-4 py-6 text-sm text-[var(--faint)]">No recipes yet.</div>}
            {recipes.map((r) => (
              <button
                key={r.name}
                onClick={() => load(r.name)}
                className={`flex w-full items-center justify-between border-t border-[var(--border)] px-4 py-2.5 text-left first:border-t-0 ${
                  selectedName === r.name ? "bg-[var(--accent-soft)]" : "hover:bg-white/[0.02]"
                }`}
              >
                <span className="truncate font-mono text-[13px] text-[var(--text)]">{r.name}</span>
                <span
                  className="ml-2 h-1.5 w-1.5 shrink-0 rounded-full"
                  style={{ background: r.valid ? "var(--allow)" : "var(--deny)" }}
                />
              </button>
            ))}
          </div>

          {/* editor */}
          <div className="flex min-h-0 flex-col rounded-xl border border-[var(--border)] bg-[var(--panel)]">
            <div className="flex items-center justify-between border-b border-[var(--border)] px-4 py-2.5">
              <span className="font-mono text-[13px] text-[var(--muted)]">
                {selectedName ?? "new recipe"}{selectedName ? ".yaml" : ""}
              </span>
              <div className="flex items-center gap-2">
                {selectedName && (
                  <button
                    onClick={onDelete}
                    className="rounded-md border border-[var(--border)] px-2.5 py-1 text-xs text-[var(--muted)] hover:border-[var(--deny)] hover:text-[var(--deny)]"
                  >
                    Delete
                  </button>
                )}
                <button
                  onClick={onSave}
                  disabled={saving || !result?.valid}
                  className="rounded-md bg-[var(--accent)] px-3.5 py-1 text-xs font-medium text-[#04122b] disabled:opacity-40"
                >
                  {saving ? "saving…" : "Save"}
                </button>
              </div>
            </div>
            <textarea
              value={src}
              onChange={(e) => setSrc(e.target.value)}
              spellCheck={false}
              className="min-h-0 flex-1 resize-none bg-transparent p-4 font-mono text-[12.5px] leading-relaxed text-[var(--text)] outline-none"
            />
            {msg && (
              <div className="border-t border-[var(--border)] px-4 py-2 text-[12px] text-[var(--muted)]">{msg}</div>
            )}
          </div>

          {/* validation */}
          <div className="overflow-auto rounded-xl border border-[var(--border)] bg-[var(--panel)] p-4">
            <Validation result={result} />
          </div>
        </div>
      </div>
    </>
  );
}

function Validation({ result }: { result: ValidateResult | null }) {
  if (!result) return <div className="text-sm text-[var(--faint)]">validating…</div>;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <span className="text-xs uppercase tracking-wider text-[var(--faint)]">Linter</span>
        <span
          className="rounded-md px-2 py-0.5 text-[11px] font-semibold"
          style={{
            color: result.valid ? "var(--allow)" : "var(--deny)",
            backgroundColor: result.valid ? "var(--allow-soft)" : "var(--deny-soft)",
          }}
        >
          {result.valid ? "VALID" : "INVALID"}
        </span>
      </div>

      {!result.valid && result.error && (
        <div className="rounded-lg border border-[var(--deny-soft)] bg-[var(--deny-soft)] px-3 py-2.5 font-mono text-[12px] leading-relaxed text-[var(--deny)]">
          {result.error}
        </div>
      )}

      {result.valid && (
        <>
          {result.hash && (
            <div>
              <div className="text-xs text-[var(--faint)]">Semantic hash</div>
              <div className="mt-0.5 font-mono text-[12px] text-[var(--text)]">sha256:{result.hash.slice(0, 16)}…</div>
            </div>
          )}
          <div>
            <div className="mb-2 text-xs uppercase tracking-wider text-[var(--faint)]">Tier preview</div>
            <div className="flex flex-col gap-1.5">
              {(result.tiers ?? []).length === 0 && (
                <div className="text-[12px] text-[var(--faint)]">No labels in the rule sets.</div>
              )}
              {(result.tiers ?? []).map((t: TierRow) => {
                const tier = TIER[t.tier] ?? TIER.deny;
                return (
                  <div key={t.label} className="flex items-center justify-between">
                    <span className="font-mono text-[12.5px] text-[var(--text)]">{t.label}</span>
                    <span
                      className="rounded px-1.5 py-0.5 text-[10px] font-semibold tracking-wide"
                      style={{ color: tier.color, backgroundColor: tier.soft }}
                    >
                      {tier.label}
                    </span>
                  </div>
                );
              })}
            </div>
          </div>
          {(result.warnings ?? []).length > 0 && (
            <div>
              <div className="mb-1.5 text-xs uppercase tracking-wider text-[var(--faint)]">Warnings</div>
              {(result.warnings ?? []).map((w, i) => (
                <div key={i} className="text-[12px] text-[var(--escalate)]">
                  • {w}
                </div>
              ))}
            </div>
          )}
        </>
      )}
    </div>
  );
}
