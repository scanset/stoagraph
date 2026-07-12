"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { getLog, setToken, type LogView } from "../lib/api";
import { setOperatorToken } from "../lib/harness";
import { adoptLoginFromURL, isLoggedIn, signOut } from "../lib/session";

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
  const [authed, setAuthed] = useState(false);

  // One-click login: if `stoagraph up`'s link put keys in the URL fragment, adopt them and drop them
  // from the address bar. Then reflect whether we are signed in.
  useEffect(() => {
    if (adoptLoginFromURL()) setReload((n) => n + 1);
    setAuthed(isLoggedIn());
  }, [reload]);

  useEffect(() => {
    getLog().then(setLog).catch(() => setLog(null));
  }, [pathname, reload]);

  return (
    <aside className="flex w-[228px] shrink-0 flex-col border-r border-[var(--border)] bg-[var(--panel)]">
      <div className="flex h-14 items-center gap-2.5 border-b border-[var(--border)] px-5">
        <Logo />
        <span className="text-[15px] font-semibold tracking-tight">
          Stoa<span className="text-[var(--accent)]">Graph</span>
        </span>
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
        <SessionBox authed={authed} connected={!!log} onChange={() => setReload((n) => n + 1)} />
        <div className="mt-3 rounded-lg border border-[var(--border)] bg-[var(--panel-2)] p-3">
          <div className="flex items-center gap-2 text-xs text-[var(--muted)]">
            <span className="h-1.5 w-1.5 rounded-full" style={{ background: log ? "var(--allow)" : "var(--deny)" }} />
            gating MCP proxy · self-hosted
          </div>
          <div className="mt-1.5 font-mono text-[11px] text-[var(--faint)]">
            {log ? `${log.verify.count} decisions · ${log.verify.signed ? "signed" : "rung-1"}` : authed ? "no data yet" : "sign in to see records"}
          </div>
        </div>
      </div>
    </aside>
  );
}

/* The whole login story, in one box. Normally the human never touches it: `stoagraph up` prints a link
 * that carries the two keys in the URL fragment, the console adopts them, and this shows "signed in".
 * The manual fallback exists only for a device where you cannot use that link. */
function SessionBox({ authed, connected, onChange }: { authed: boolean; connected: boolean; onChange: () => void }) {
  const [manual, setManual] = useState(false);

  if (authed) {
    return (
      <div className="rounded-lg border border-[var(--border)] bg-[var(--panel-2)] p-3">
        <div className="flex items-center justify-between">
          <span className="flex items-center gap-2 text-xs text-[var(--muted)]">
            <span
              className="h-1.5 w-1.5 rounded-full"
              style={{ background: connected ? "var(--allow)" : "var(--escalate)" }}
            />
            {connected ? "signed in" : "signed in · gate unreachable"}
          </span>
          <button
            onClick={() => {
              signOut();
              onChange();
            }}
            className="text-[11px] text-[var(--faint)] transition hover:text-[var(--deny)]"
          >
            sign out
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-[var(--border)] bg-[var(--panel-2)] p-3">
      <div className="text-xs font-medium text-[var(--muted)]">Not signed in</div>
      <p className="mt-1.5 text-[11px] leading-relaxed text-[var(--faint)]">
        Run <code className="text-[var(--text)]">stoagraph up</code> (or{" "}
        <code className="text-[var(--text)]">stoagraph console</code>) and open the link it prints.
      </p>
      <button
        onClick={() => setManual((m) => !m)}
        className="mt-1.5 text-[11px] text-[var(--faint)] underline-offset-2 hover:underline"
      >
        {manual ? "hide" : "paste keys manually"}
      </button>
      {manual && <ManualLogin onDone={onChange} />}
    </div>
  );
}

function ManualLogin({ onDone }: { onDone: () => void }) {
  const [c, setC] = useState("");
  const [o, setO] = useState("");
  return (
    <div className="mt-2 flex flex-col gap-1.5">
      <input
        type="password"
        value={c}
        onChange={(e) => setC(e.target.value)}
        placeholder="console key"
        autoComplete="off"
        spellCheck={false}
        className="w-full rounded border border-[var(--border)] bg-[var(--panel)] px-2 py-1 font-mono text-[11px] text-[var(--text)] outline-none focus:border-[var(--accent)]"
      />
      <input
        type="password"
        value={o}
        onChange={(e) => setO(e.target.value)}
        placeholder="operator key"
        autoComplete="off"
        spellCheck={false}
        className="w-full rounded border border-[var(--border)] bg-[var(--panel)] px-2 py-1 font-mono text-[11px] text-[var(--text)] outline-none focus:border-[var(--accent)]"
      />
      <button
        onClick={() => {
          if (c) setToken(c.trim());
          if (o) setOperatorToken(o.trim());
          onDone();
        }}
        className="rounded border border-[var(--border)] px-2 py-1 text-[11px] text-[var(--muted)] transition hover:border-[var(--accent)] hover:text-[var(--text)]"
      >
        save
      </button>
    </div>
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
  // The mark is white line-art on black (a stoa, with the gate/graph node between its columns). The
  // black square reads as intentional inside a bordered tile — no transparency needed.
  return (
    <span className="grid h-7 w-7 place-items-center overflow-hidden rounded-md border border-[var(--border-strong)] bg-black">
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img src="/stoa-graph-logo.png" alt="StoaGraph" width={28} height={28} className="h-7 w-7" />
    </span>
  );
}
