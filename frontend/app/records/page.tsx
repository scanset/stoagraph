"use client";

import { useCallback, useEffect, useState } from "react";
import { getLog, type LogView } from "../lib/api";
import { isLoggedIn } from "../lib/session";

/* Records — the audit trail. Every row is a moment the gate RELEASED an untrusted value to an
 * authoritative sink: which field it crossed, the rule that authorized it, the policy that owns the
 * rule. The whole thing is hash-chained (and optionally signed), so it is tamper-EVIDENT: if a byte is
 * altered, verification fails and this page says so. This is the "recorded as proof" half of the
 * product — the deterministic record that policy, not a model, allowed each crossing. */
export default function RecordsPage() {
  const [log, setLog] = useState<LogView | null>(null);
  const [authed, setAuthed] = useState(true);
  const [offline, setOffline] = useState(false);

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
    const id = setInterval(refresh, 4000); // crossings accrue as the gate decides; keep it live
    return () => clearInterval(id);
  }, [refresh]);

  const v = log?.verify;
  const events = log?.events ?? [];
  // newest first — the log is append-only, so the last entries are the most recent crossings
  const rows = [...events].reverse();

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
          Every row is a crossing the gate <span className="text-[var(--text)]">cleared</span>: an
          untrusted value released to an authoritative sink, and the rule that authorized it. The log is
          hash-chained — alter one byte and verification fails. Decided by policy, not a model, and
          recorded as proof.
        </p>

        {/* chain header */}
        {v && (
          <div className="mb-5 grid grid-cols-2 gap-3 sm:grid-cols-4">
            <Stat label="crossings" value={String(v.count)} />
            <Stat
              label="integrity"
              value={v.error ? "TAMPERED" : v.verified ? "verified" : "—"}
              tone={v.error ? "var(--deny)" : v.verified ? "var(--allow)" : undefined}
            />
            <Stat label={v.signed ? `signed · ${v.keyId ?? "ed25519"}` : "signed"} value={v.signed ? "yes" : "no"} />
            <Stat label="chain head" value={v.head ? v.head.slice(0, 12) + "…" : "—"} mono title={v.head} />
          </div>
        )}

        {v?.error && (
          <div className="mb-5 rounded-lg border border-[var(--deny)] bg-[var(--deny-soft)] px-4 py-3 text-[12.5px] text-[var(--deny)]">
            ⚠ {v.error}
          </div>
        )}

        {/* crossings */}
        {rows.length === 0 ? (
          <div className="rounded-xl border border-[var(--border)] bg-[var(--panel)] px-5 py-12 text-center text-sm text-[var(--faint)]">
            {authed && !offline
              ? "No crossings recorded yet. A row appears here each time the gate releases a value to an authoritative sink."
              : status.label}
          </div>
        ) : (
          <div className="overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--panel)]">
            <div className="grid grid-cols-[1.6fr_1fr_1.2fr_auto] gap-3 border-b border-[var(--border)] px-4 py-2.5 text-[10.5px] font-medium uppercase tracking-wider text-[var(--faint)]">
              <span>field released to</span>
              <span>authorizing rule</span>
              <span>policy</span>
              <span>subject</span>
            </div>
            {rows.map((e, i) => (
              <div
                key={i}
                className="grid grid-cols-[1.6fr_1fr_1.2fr_auto] items-center gap-3 border-b border-[var(--border)] px-4 py-2.5 last:border-b-0 hover:bg-[var(--panel-2)]"
              >
                <span className="truncate font-mono text-[12px] text-[var(--text)]">{e.field}</span>
                <span className="truncate font-mono text-[12px] text-[var(--accent)]">{e.rule}</span>
                <span className="truncate font-mono text-[11.5px] text-[var(--muted)]">{e.actor}</span>
                <span
                  className="justify-self-end rounded px-1.5 py-0.5 text-[10px] font-medium"
                  style={{ color: "var(--escalate)", backgroundColor: "var(--escalate-soft)" }}
                >
                  {e.subject}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </>
  );
}

function Stat({
  label,
  value,
  tone,
  mono,
  title,
}: {
  label: string;
  value: string;
  tone?: string;
  mono?: boolean;
  title?: string;
}) {
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--panel)] px-4 py-3" title={title}>
      <div className="text-[10.5px] uppercase tracking-wider text-[var(--faint)]">{label}</div>
      <div
        className={`mt-1 text-[15px] font-semibold ${mono ? "font-mono text-[12px]" : ""}`}
        style={tone ? { color: tone } : undefined}
      >
        {value}
      </div>
    </div>
  );
}
