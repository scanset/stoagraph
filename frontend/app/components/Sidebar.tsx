"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { getLog, getToken, setToken, type LogView } from "../lib/api";
import { getOperatorToken, setOperatorToken } from "../lib/harness";

// One console, two backends (Planning/26): the GATE (stag-serve) holds policy/approvals and no keys;
// the ORCHESTRATOR (harness-serve) holds the models and dispatches. Only the UI is unified — keeping
// the backends separate is what lets the gate stay independently runnable.
const NAV: [string, string][] = [
  ["Live", "/"],
  ["Dispatch", "/dispatch"],
  ["Recipes", "/recipes"],
  ["Approvals", "/approvals"],
  ["Records", "/records"],
  ["Adapters", "/adapters"],
  ["Models", "/models"],
];

export default function Sidebar() {
  const pathname = usePathname();
  const [log, setLog] = useState<LogView | null>(null);
  const [reload, setReload] = useState(0);

  useEffect(() => {
    getLog().then(setLog).catch(() => setLog(null));
  }, [pathname, reload]);

  return (
    <aside className="flex w-[228px] shrink-0 flex-col border-r border-[var(--border)] bg-[var(--panel)]">
      <div className="flex h-14 items-center gap-2.5 border-b border-[var(--border)] px-5">
        <Logo />
        <span className="font-mono text-[15px] font-semibold tracking-tight">stag</span>
      </div>
      <nav className="flex flex-col gap-0.5 p-3">
        {NAV.map(([label, href]) => {
          const active = href === "/" ? pathname === "/" : pathname.startsWith(href);
          return (
            <Link
              key={label}
              href={href}
              className={`flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition ${
                active
                  ? "bg-[var(--accent-soft)] font-medium text-[var(--text)]"
                  : "text-[var(--muted)] hover:bg-white/[0.03] hover:text-[var(--text)]"
              }`}
            >
              <span className="h-1.5 w-1.5 rounded-full" style={{ background: active ? "var(--accent)" : "var(--faint)" }} />
              {label}
            </Link>
          );
        })}
      </nav>
      <div className="mt-auto border-t border-[var(--border)] p-4">
        <TokenBox
          label="gate token"
          hint="admin / approve"
          connected={!!log}
          read={getToken}
          write={setToken}
          onSaved={() => setReload((n) => n + 1)}
        />
        <div className="mt-2">
          <TokenBox
            label="orchestrator token"
            hint="operator"
            read={getOperatorToken}
            write={setOperatorToken}
            onSaved={() => setReload((n) => n + 1)}
          />
        </div>
        <div className="mt-3 rounded-lg border border-[var(--border)] bg-[var(--panel-2)] p-3">
          <div className="flex items-center gap-2 text-xs text-[var(--muted)]">
            <span className="h-1.5 w-1.5 rounded-full" style={{ background: log ? "var(--allow)" : "var(--deny)" }} />
            gating MCP proxy · self-hosted
          </div>
          <div className="mt-1.5 font-mono text-[11px] text-[var(--faint)]">
            {log ? `${log.verify.count} crossings · ${log.verify.signed ? "signed" : "rung-1"}` : "no data — check token"}
          </div>
        </div>
      </div>
    </aside>
  );
}

/* The console is the HUMAN's tool and it talks to TWO backends with DIFFERENT roles (Planning/31):
 *   gate token         — `admin` to author policy, `approve` to release an escalation.
 *   orchestrator token — `operator` for models / event map / dispatch.
 * Neither is the `dispatch` token: that one belongs to the orchestrator PROCESS, and it is
 * deliberately unable to approve. An orchestrator that could approve its own escalations would make
 * the human-in-the-loop gate decorative. Both secrets are generated at data/control.tokens on the
 * gate's first start. */
function TokenBox({
  label,
  hint,
  connected,
  read,
  write,
  onSaved,
}: {
  label: string;
  hint: string;
  connected?: boolean;
  read: () => string;
  write: (t: string) => void;
  onSaved: () => void;
}) {
  const [value, setValue] = useState("");
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    setValue(read());
  }, [read]);

  const save = () => {
    write(value.trim());
    setSaved(true);
    onSaved();
    setTimeout(() => setSaved(false), 1500);
  };

  return (
    <div className="rounded-lg border border-[var(--border)] bg-[var(--panel-2)] p-3">
      <label className="flex items-center gap-2 text-xs text-[var(--muted)]">
        {connected !== undefined && (
          <span
            className="h-1.5 w-1.5 rounded-full"
            style={{ background: connected ? "var(--allow)" : "var(--deny)" }}
          />
        )}
        {label}
      </label>
      <input
        type="password"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => e.key === "Enter" && save()}
        placeholder={hint}
        spellCheck={false}
        autoComplete="off"
        className="mt-2 w-full rounded border border-[var(--border)] bg-[var(--panel)] px-2 py-1 font-mono text-[11px] text-[var(--text)] outline-none focus:border-[var(--accent)]"
      />
      <button
        onClick={save}
        className="mt-1.5 w-full rounded border border-[var(--border)] px-2 py-1 text-[11px] text-[var(--muted)] transition hover:border-[var(--accent)] hover:text-[var(--text)]"
      >
        {saved ? "saved" : "save token"}
      </button>
    </div>
  );
}

function Logo() {
  return (
    <span className="grid h-7 w-7 place-items-center rounded-md border border-[var(--border-strong)] bg-[var(--panel-2)]">
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" aria-hidden>
        <path d="M12 3 4 6v6c0 4.4 3.2 7.6 8 10 4.8-2.4 8-5.6 8-10V6l-8-3Z" stroke="var(--accent)" strokeWidth="1.6" strokeLinejoin="round" />
        <circle cx="12" cy="11" r="2" fill="var(--allow)" />
        <path d="M12 13v3M12 11 8.5 8.5M12 11l3.5-2.5" stroke="var(--allow)" strokeWidth="1.3" strokeLinecap="round" />
      </svg>
    </span>
  );
}
