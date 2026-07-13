"use client";

import Link from "next/link";

/* The first thing you see on an empty gate. A fresh gate permits NOTHING — which is correct, but an
 * empty screen doesn't say that or tell you what to do. This turns the blank state into a path: wire
 * your own tool in four steps. */
export default function GetStarted() {
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
          something you didn&rsquo;t author. Wire your first tool to see the gate work.
        </p>

        {/* the manual path */}
        <div className="mt-7">
          <div className="text-xs font-medium uppercase tracking-wider text-[var(--faint)]">wire your first tool</div>
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
