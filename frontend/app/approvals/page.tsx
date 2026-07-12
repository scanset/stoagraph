"use client";

import { useCallback, useEffect, useState } from "react";
import { approveApproval, denyApproval, listApprovals, type ApprovalView } from "../lib/api";
import { isLoggedIn } from "../lib/session";

const STATUS: Record<string, { label: string; color: string; soft: string }> = {
  pending: { label: "PENDING", color: "var(--escalate)", soft: "var(--escalate-soft)" },
  approved: { label: "APPROVED", color: "var(--allow)", soft: "var(--allow-soft)" },
  denied: { label: "DENIED", color: "var(--deny)", soft: "var(--deny-soft)" },
  consumed: { label: "RELEASED", color: "var(--muted)", soft: "rgba(255,255,255,0.05)" },
};

type Filter = "pending" | "all";

export default function ApprovalsPage() {
  const [rows, setRows] = useState<ApprovalView[]>([]);
  const [filter, setFilter] = useState<Filter>("pending");
  const [busy, setBusy] = useState<string | null>(null);
  const [minted, setMinted] = useState<Record<string, string>>({}); // id -> "keyId" note after approve
  const [offline, setOffline] = useState(false);
  const [authed, setAuthed] = useState(true);

  const refresh = useCallback(() => {
    // Not signed in is not "offline" — do not send an operator hunting for a dead server.
    setAuthed(isLoggedIn());
    if (!isLoggedIn()) return;
    listApprovals(filter === "pending" ? "pending" : undefined)
      .then((r) => {
        setRows(r);
        setOffline(false);
      })
      .catch(() => setOffline(true));
  }, [filter]);

  // poll: escalations arrive from the gate out-of-band, so keep the queue live.
  useEffect(() => {
    refresh();
    const id = setInterval(refresh, 3000);
    return () => clearInterval(id);
  }, [refresh]);

  const onApprove = useCallback(
    async (id: string) => {
      setBusy(id);
      try {
        const { ok, data } = await approveApproval(id);
        if (ok && data.token) setMinted((m) => ({ ...m, [id]: data.keyId }));
        refresh();
      } finally {
        setBusy(null);
      }
    },
    [refresh],
  );

  const onDeny = useCallback(
    async (id: string) => {
      setBusy(id);
      try {
        await denyApproval(id);
        refresh();
      } finally {
        setBusy(null);
      }
    },
    [refresh],
  );

  const pendingCount = rows.filter((r) => r.status === "pending").length;

  return (
    <>
      <header className="flex h-14 shrink-0 items-center justify-between border-b border-[var(--border)] px-6">
        <div className="flex items-baseline gap-3">
          <h1 className="text-[15px] font-semibold tracking-tight">Approvals</h1>
          <span className="text-sm text-[var(--faint)]">
            {!authed ? "sign in to view approvals" : offline ? "backend offline" : `${pendingCount} awaiting a signed release`}
          </span>
        </div>
        <div className="flex items-center gap-1 rounded-lg border border-[var(--border)] p-0.5">
          {(["pending", "all"] as Filter[]).map((f) => (
            <button
              key={f}
              onClick={() => setFilter(f)}
              className={`rounded-md px-3 py-1 text-xs capitalize ${
                filter === f ? "bg-[var(--accent-soft)] text-[var(--text)]" : "text-[var(--muted)] hover:text-[var(--text)]"
              }`}
            >
              {f}
            </button>
          ))}
        </div>
      </header>

      <div className="flex-1 overflow-auto p-6">
        <p className="mb-4 max-w-3xl text-[13px] leading-relaxed text-[var(--faint)]">
          When the gate <span className="text-[var(--escalate)]">escalates</span> an action, it lands here. Approving mints a{" "}
          <span className="text-[var(--text)]">signed release</span> (ed25519, bound to the exact action) — the orchestrator&rsquo;s
          retry then passes the recipe&rsquo;s <span className="font-mono">signed_equality</span> gate. Releases are one-time: a
          replayed token re-escalates.
        </p>

        {rows.length === 0 && (
          <div className="rounded-xl border border-[var(--border)] bg-[var(--panel)] px-5 py-10 text-center text-sm text-[var(--faint)]">
            {filter === "pending" ? "Nothing awaiting approval." : "No approvals recorded yet."}
          </div>
        )}

        <div className="flex flex-col gap-3">
          {rows.map((a) => {
            const s = STATUS[a.status] ?? STATUS.denied;
            const isPending = a.status === "pending";
            return (
              <div
                key={a.id}
                className="rounded-xl border border-[var(--border)] bg-[var(--panel)] p-4"
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2.5">
                      <span className="font-mono text-[13.5px] font-medium text-[var(--text)]">{a.tool}</span>
                      <span
                        className="rounded px-1.5 py-0.5 text-[10px] font-semibold tracking-wide"
                        style={{ color: s.color, backgroundColor: s.soft }}
                      >
                        {s.label}
                      </span>
                      {a.recipe && <span className="font-mono text-[11px] text-[var(--faint)]">{a.recipe}</span>}
                    </div>
                    <div className="mt-2 flex flex-wrap gap-1.5">
                      {Object.entries(a.args).map(([k, v]) => (
                        <span
                          key={k}
                          className="rounded-md border border-[var(--border)] bg-[var(--panel-2)] px-2 py-0.5 font-mono text-[11.5px] text-[var(--muted)]"
                        >
                          {k}=<span className="text-[var(--text)]">{v}</span>
                        </span>
                      ))}
                    </div>
                    <div className="mt-2 font-mono text-[10.5px] text-[var(--faint)]">
                      {a.id} · {a.createdAt ?? ""}
                      {a.reason ? ` · “${a.reason}”` : ""}
                      {a.tokenIssued && !minted[a.id] ? " · signed release issued" : ""}
                      {minted[a.id] ? ` · signed · key ${minted[a.id]}` : ""}
                    </div>
                  </div>
                  {isPending && (
                    <div className="flex shrink-0 items-center gap-2">
                      <button
                        onClick={() => onDeny(a.id)}
                        disabled={busy === a.id}
                        className="rounded-md border border-[var(--border)] px-3 py-1.5 text-xs text-[var(--muted)] hover:border-[var(--deny)] hover:text-[var(--deny)] disabled:opacity-40"
                      >
                        Deny
                      </button>
                      <button
                        onClick={() => onApprove(a.id)}
                        disabled={busy === a.id}
                        className="rounded-md bg-[var(--accent)] px-3.5 py-1.5 text-xs font-medium text-[#04122b] disabled:opacity-40"
                      >
                        {busy === a.id ? "…" : "Approve"}
                      </button>
                    </div>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </>
  );
}
