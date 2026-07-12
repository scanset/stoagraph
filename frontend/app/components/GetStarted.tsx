"use client";

import Link from "next/link";
import { useState } from "react";
import { loadDemo } from "../lib/demo";

/* The first thing you see on an empty gate. A fresh gate permits NOTHING — which is correct, but an
 * empty screen doesn't say that or tell you what to do. This turns the blank state into a path: run the
 * demo to see it work in one click, or wire your own tool in four steps. */
export default function GetStarted({ onLoaded }: { onLoaded: () => void }) {
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState<string[] | null>(null);
  const [err, setErr] = useState<string | null>(null);

  const runDemo = async () => {
    setBusy(true);
    setErr(null);
    try {
      setDone(await loadDemo());
      onLoaded();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="mx-auto max-w-2xl py-10">
      <div className="rounded-2xl border border-[var(--border)] bg-[var(--panel)] p-8">
        <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-wider text-[var(--faint)]">
          <span className="h-1.5 w-1.5 rounded-full" style={{ background: "var(--escalate)" }} />
          your gate is empty
        </div>
        <h1 className="mt-3 text-2xl font-semibold tracking-tight">Nothing is permitted yet.</h1>
        <p className="mt-2 text-[13.5px] leading-relaxed text-[var(--muted)]">
          That is the correct starting point for a security control — it never arrives already allowing
          something you didn&rsquo;t author. Load the demo to watch it work, or wire your own tool.
        </p>

        {/* one-click demo */}
        <div className="mt-6 rounded-xl border border-[var(--border-strong)] bg-[var(--panel-2)] p-5">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              <div className="text-sm font-semibold">Load the containment demo</div>
              <p className="mt-1 text-[12.5px] leading-relaxed text-[var(--faint)]">
                A support agent that can read a customer&rsquo;s record — SSN and all — but can only send
                replies that match an approved template. No model or API key needed.
              </p>
            </div>
            <button
              onClick={runDemo}
              disabled={busy || !!done}
              className="shrink-0 rounded-lg px-4 py-2 text-sm font-medium text-[#04122b] disabled:opacity-50"
              style={{ background: "linear-gradient(180deg, var(--accent), var(--accent-2))" }}
            >
              {busy ? "loading…" : done ? "loaded ✓" : "Load demo"}
            </button>
          </div>
          {done && (
            <div className="mt-3 flex flex-wrap gap-1.5">
              {done.map((s) => (
                <span key={s} className="rounded-md bg-[var(--allow-soft)] px-2 py-0.5 text-[11px] text-[var(--allow)]">
                  {s}
                </span>
              ))}
              <span className="text-[11px] text-[var(--faint)]">— now try a call in Live, or watch a real agent in Dispatch.</span>
            </div>
          )}
          {err && <div className="mt-3 text-[12px] text-[var(--deny)]">⚠ {err}</div>}
        </div>

        {/* the manual path */}
        <div className="mt-7">
          <div className="text-xs font-medium uppercase tracking-wider text-[var(--faint)]">or wire your own tool</div>
          <ol className="mt-3 flex flex-col gap-2.5">
            <Step n={1} to="/adapters" title="Register a tool server" sub="Point the gate at an MCP server — yours, or the example one." />
            <Step n={2} to="/recipes" title="Write a policy" sub="A recipe: which arguments may take which values." />
            <Step n={3} to="/adapters" title="Route a tool to it" sub="Bind the tool to the recipe and name the gated argument." />
            <Step n={4} to="/" title="Try it in Live" sub="Propose a call and watch the gate allow or deny it." />
          </ol>
          <p className="mt-4 text-[12px] text-[var(--faint)]">
            New to writing recipes? See{" "}
            <span className="font-mono text-[var(--muted)]">examples/custom-tool/</span> — a copy-paste
            tool server and a 12-line policy.
          </p>
        </div>
      </div>
    </div>
  );
}

function Step({ n, to, title, sub }: { n: number; to: string; title: string; sub: string }) {
  return (
    <li>
      <Link
        href={to}
        className="flex items-center gap-3.5 rounded-xl border border-[var(--border)] bg-[var(--panel-2)] px-4 py-3 transition hover:border-[var(--border-strong)] hover:bg-[var(--panel-3)]"
      >
        <span className="grid h-7 w-7 shrink-0 place-items-center rounded-full border border-[var(--border-strong)] font-mono text-[12px] text-[var(--muted)]">
          {n}
        </span>
        <span className="min-w-0">
          <span className="block text-[13.5px] font-medium">{title}</span>
          <span className="block text-[12px] text-[var(--faint)]">{sub}</span>
        </span>
        <span className="ml-auto text-[var(--faint)]">→</span>
      </Link>
    </li>
  );
}
