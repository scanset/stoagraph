"use client";

import { useCallback, useEffect, useState } from "react";
import { getLog, type LogView, type RecordView } from "../lib/api";
import { isLoggedIn } from "../lib/session";

/* Records — the audit trail. One row per DECISION the gate made: allowed, denied, or escalated.
 *
 * Denials are recorded, not just allows. "The agent reached for a repo it was never granted, and was
 * blocked by owner.allowed" is precisely the evidence an auditor wants — a log of only the permitted
 * actions cannot answer "did anything try?", which is the question it exists to answer.
 *
 * A RELEASE is a crossing that actually happened: an untrusted value that reached an authoritative sink.
 * A denied call releases nothing, so it shows none. The chain is hash-linked — alter one byte and
 * verification fails, and this page says so. */
export default function RecordsPage() {
  const [log, setLog] = useState<LogView | null>(null);
  const [authed, setAuthed] = useState(true);
  const [offline, setOffline] = useState(false);
  const [filter, setFilter] = useState<"all" | "allow" | "blocked">("all");

  const refresh = useCallback(() => {
    setAuthed(isLoggedIn());
    if (!isLoggedIn()) return;
    getLog()
      .then((l) => {
        setLog(l);
        setOffline(false);
      })
      .catch(() => setOffline(true));
  }, []);

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, 4000);
    return () => clearInterval(id);
  }, [refresh]);

  const v = log?.verify;
  const all = [...(log?.records ?? [])].reverse(); // newest first
  const rows = all.filter((r) =>
    filter === "all" ? true : filter === "allow" ? r.verdict === "allow" : r.verdict !== "allow",
  );
  const allowed = all.filter((r) => r.verdict === "allow").length;
  const blocked = all.length - allowed;
  const releases = all.reduce((n, r) => n + (r.releases?.length ?? 0), 0);

  const status = !authed
    ? { label: "sign in to view records", tone: "var(--escalate)" }
    : offline
      ? { label: "backend offline", tone: "var(--deny)" }
      : v?.error
        ? { label: "TAMPERED — chain verification failed", tone: "var(--deny)" }
        : v?.verified
          ? { label: "chain intact", tone: "var(--allow)" }
          : { label: "…", tone: "var(--faint)" };

  return (
    <>
      <header className="flex h-14 shrink-0 items-center justify-between border-b border-[var(--border)] px-6">
        <div className="flex items-baseline gap-3">
          <h1 className="text-[15px] font-semibold tracking-tight">Records</h1>
          <span className="text-sm text-[var(--faint)]">the tamper-evident audit log</span>
        </div>
        <span className="flex items-center gap-2 text-xs text-[var(--muted)]">
          <span className="h-1.5 w-1.5 rounded-full" style={{ background: status.tone }} />
          {status.label}
        </span>
      </header>

      <div className="flex-1 overflow-auto p-6">
        <p className="mb-4 max-w-3xl text-[13px] leading-relaxed text-[var(--faint)]">
          One row per decision the gate made — <span className="text-[var(--allow)]">allowed</span> and{" "}
          <span className="text-[var(--deny)]">blocked</span> alike. A blocked attempt is the evidence the
          control worked. A <span className="text-[var(--text)]">release</span> is a crossing that actually
          happened; a denied call releases nothing. Hash-chained — alter one byte and verification fails.
        </p>

        {v && (
          <div className="mb-5 grid grid-cols-2 gap-3 sm:grid-cols-5">
            <Stat label="decisions" value={String(v.count)} />
            <Stat label="allowed" value={String(allowed)} tone="var(--allow)" />
            <Stat label="blocked" value={String(blocked)} tone={blocked ? "var(--deny)" : undefined} />
            <Stat label="releases" value={String(releases)} />
            <Stat
              label="integrity"
              value={v.error ? "TAMPERED" : v.verified ? "verified" : "—"}
              tone={v.error ? "var(--deny)" : v.verified ? "var(--allow)" : undefined}
              title={v.head}
            />
          </div>
        )}

        {v?.error && (
          <div className="mb-5 rounded-lg border border-[var(--deny)] bg-[var(--deny-soft)] px-4 py-3 text-[12.5px] text-[var(--deny)]">
            ⚠ {v.error}
          </div>
        )}

        {all.length > 0 && (
          <div className="mb-3 flex gap-1.5">
            {(["all", "allow", "blocked"] as const).map((f) => (
              <button
                key={f}
                onClick={() => setFilter(f)}
                className={`rounded-md border px-2.5 py-1 text-[11.5px] ${
                  filter === f
                    ? "border-[var(--border-strong)] bg-[var(--panel-2)] text-[var(--text)]"
                    : "border-[var(--border)] text-[var(--faint)] hover:text-[var(--muted)]"
                }`}
              >
                {f}
              </button>
            ))}
          </div>
        )}

        {rows.length === 0 ? (
          <div className="rounded-xl border border-[var(--border)] bg-[var(--panel)] px-5 py-12 text-center text-sm text-[var(--faint)]">
            {authed && !offline
              ? "No decisions recorded yet. A row appears here every time the gate decides a tool call — allowed or blocked."
              : status.label}
          </div>
        ) : (
          <div className="flex flex-col gap-2">
            {rows.map((r, i) => (
              <Row key={i} r={r} />
            ))}
          </div>
        )}
      </div>
    </>
  );
}

function Row({ r }: { r: RecordView }) {
  const tone =
    r.verdict === "allow" ? "var(--allow)" : r.verdict === "escalate" ? "var(--escalate)" : "var(--deny)";
  const soft =
    r.verdict === "allow"
      ? "var(--allow-soft)"
      : r.verdict === "escalate"
        ? "var(--escalate-soft)"
        : "var(--deny-soft)";

  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--panel)] px-4 py-3">
      <div className="flex flex-wrap items-center gap-2">
        <span
          className="rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide"
          style={{ color: tone, backgroundColor: soft }}
        >
          {r.verdict}
        </span>
        <span className="font-mono text-[13px] text-[var(--text)]">{r.tool}</span>
        {r.value && <span className="truncate font-mono text-[12px] text-[var(--muted)]">{r.value}</span>}
        {r.recipe && (
          <span className="ml-auto rounded bg-[var(--panel-3)] px-1.5 py-0.5 font-mono text-[10px] text-[var(--faint)]">
            {r.recipe}
          </span>
        )}
      </div>

      {/* why it was refused */}
      {r.verdict !== "allow" && (
        <div className="mt-1.5 text-[11.5px] text-[var(--deny)]">
          not forwarded{r.fault ? ` — ${r.fault}` : ""}
        </div>
      )}

      {/* what actually crossed (forwarded calls only) */}
      {r.releases && r.releases.length > 0 && (
        <div className="mt-2 border-t border-[var(--border)] pt-2">
          <div className="mb-1 text-[10px] uppercase tracking-wider text-[var(--faint)]">
            released {r.releases.length === 1 ? "1 crossing" : `${r.releases.length} crossings`}
          </div>
          {r.releases.map((e, i) => (
            <div key={i} className="flex flex-wrap items-center gap-2 py-0.5 text-[11.5px]">
              <span className="font-mono text-[var(--text)]">{e.field}</span>
              <span className="text-[var(--faint)]">←</span>
              <span className="font-mono text-[var(--accent)]">{e.rule}</span>
              <span className="font-mono text-[10.5px] text-[var(--muted)]">{e.actor}</span>
              <span
                className="rounded px-1 py-0.5 text-[10px]"
                style={{ color: "var(--escalate)", backgroundColor: "var(--escalate-soft)" }}
              >
                {e.subject}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function Stat({ label, value, tone, title }: { label: string; value: string; tone?: string; title?: string }) {
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--panel)] px-4 py-3" title={title}>
      <div className="text-[10.5px] uppercase tracking-wider text-[var(--faint)]">{label}</div>
      <div className="mt-1 text-[15px] font-semibold" style={tone ? { color: tone } : undefined}>
        {value}
      </div>
    </div>
  );
}
