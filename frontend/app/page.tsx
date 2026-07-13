"use client";

import { isLoggedIn } from "./lib/session";
import { isGateEmpty } from "./lib/gate";
import GetStarted from "./components/GetStarted";
import { useCallback, useEffect, useState } from "react";
import {
  decide,
  getLog,
  getPolicies,
  type DecisionView,
  type LogView,
  type PolicyView,
  type Verdict,
} from "./lib/api";

/* ------------------------------- data ------------------------------- */

type Decision = DecisionView & { t: string };

const V: Record<Verdict, { label: string; color: string; soft: string }> = {
  allow: { label: "ALLOW", color: "var(--allow)", soft: "var(--allow-soft)" },
  deny: { label: "DENY", color: "var(--deny)", soft: "var(--deny-soft)" },
  escalate: { label: "ESCALATE", color: "var(--escalate)", soft: "var(--escalate-soft)" },
};

function nowStamp(): string {
  const d = new Date();
  return d.toLocaleTimeString("en-US", { hour12: false }) + "." + String(d.getMilliseconds()).padStart(3, "0");
}

/* ------------------------------- page ------------------------------- */

export default function Page() {
  const [policy, setPolicy] = useState<PolicyView | null>(null);
  const [decisions, setDecisions] = useState<Decision[]>([]);
  const [selected, setSelected] = useState(0);
  const [log, setLog] = useState<LogView | null>(null);
  const [value, setValue] = useState("hello");
  const [busy, setBusy] = useState(false);
  const [connected, setConnected] = useState<boolean | null>(null);
  const [authed, setAuthed] = useState(true);
  const [empty, setEmpty] = useState<boolean | null>(null);
  const [err, setErr] = useState<string | null>(null);

  const refreshLog = useCallback(() => {
    getLog().then(setLog).catch(() => {});
  }, []);

  const load = useCallback(() => {
    // A 401 means "not signed in", which is NOT the same as "backend offline" — reporting the former
    // as the latter sends people hunting for a dead server when they just need to log in.
    setAuthed(isLoggedIn());
    if (!isLoggedIn()) return;
    getPolicies()
      .then((p) => {
        setPolicy(p[0] ?? null);
        setConnected(true);
      })
      .catch(() => setConnected(false));
    isGateEmpty().then(setEmpty).catch(() => setEmpty(false));
    refreshLog();
  }, [refreshLog]);

  useEffect(() => load(), [load]);

  const submit = useCallback(async () => {
    if (!policy || busy) return;
    setBusy(true);
    setErr(null);
    try {
      const d = await decide(policy.tool, { [policy.gateArg]: value });
      setDecisions((prev) => [{ ...d, t: nowStamp() }, ...prev]);
      setSelected(0);
      setConnected(true);
      refreshLog();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
      setConnected(false);
    } finally {
      setBusy(false);
    }
  }, [policy, value, busy, refreshLog]);

  const current = decisions[selected] ?? null;

  // Signed in, gate empty → onboarding instead of a blank playground. (Only when authed: an empty read
  // could just mean "not logged in", and the topbar already says "sign in" for that.)
  const showOnboarding = authed && empty === true;

  return (
    <>
      <Topbar connected={connected} authed={authed} policy={policy} />
      <div className="flex-1 overflow-auto p-6">
        {showOnboarding ? (
          <GetStarted />
        ) : (
          <>
            <DecideBar
              policy={policy}
              value={value}
              setValue={setValue}
              onSubmit={submit}
              busy={busy}
              err={err}
            />
            <div className="mt-5">
              <Stats decisions={decisions} log={log} />
            </div>
            <div className="mt-6 grid grid-cols-1 gap-6 xl:grid-cols-[1.55fr_1fr]">
              <Feed decisions={decisions} selected={selected} onSelect={setSelected} />
              <Detail decision={current} log={log} onVerify={refreshLog} />
            </div>
          </>
        )}
      </div>
    </>
  );
}

/* ------------------------------ topbar ------------------------------ */

function Topbar({
  connected,
  authed,
  policy,
}: {
  connected: boolean | null;
  authed: boolean;
  policy: PolicyView | null;
}) {
  // Not signed in is its own state — a red "backend offline" here is a lie that costs someone an hour.
  const status = !authed
    ? "sign in — run `stoagraph up` and open the link"
    : connected === false
      ? "backend offline"
      : connected
        ? "connected"
        : "connecting…";
  const good = authed && connected !== false;
  return (
    <header className="flex h-14 shrink-0 items-center justify-between border-b border-[var(--border)] px-6">
      <div className="flex items-baseline gap-3">
        <h1 className="text-[15px] font-semibold tracking-tight">Live gating</h1>
        <span className="text-sm text-[var(--faint)]">
          {policy ? `tool · ${policy.tool}` : "no policy loaded"}
        </span>
      </div>
      <div className="flex items-center gap-2 text-xs text-[var(--muted)]">
        <span className="relative flex h-2 w-2">
          {good && authed && connected && (
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[var(--allow)] opacity-60" />
          )}
          <span
            className="relative inline-flex h-2 w-2 rounded-full"
            style={{ background: !authed ? "var(--escalate)" : connected === false ? "var(--deny)" : "var(--allow)" }}
          />
        </span>
        {status}
      </div>
    </header>
  );
}

/* ---------------------------- decide bar ---------------------------- */

const EXAMPLES = ["hello", "status-ok", "deploy-done", "rm -rf /", "; drop table users"];

function DecideBar({
  policy,
  value,
  setValue,
  onSubmit,
  busy,
  err,
}: {
  policy: PolicyView | null;
  value: string;
  setValue: (v: string) => void;
  onSubmit: () => void;
  busy: boolean;
  err: string | null;
}) {
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--panel)] p-4">
      <div className="mb-2.5 flex items-center gap-2 text-xs uppercase tracking-wider text-[var(--faint)]">
        <span>propose a tool call</span>
        {policy && (
          <span className="rounded bg-[var(--panel-3)] px-1.5 py-0.5 font-mono text-[11px] normal-case text-[var(--muted)]">
            {policy.tool}({policy.gateArg})
          </span>
        )}
      </div>
      <div className="flex flex-wrap items-center gap-2.5">
        <input
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && onSubmit()}
          placeholder="argument value the agent proposes…"
          className="min-w-0 flex-1 rounded-lg border border-[var(--border-strong)] bg-[var(--panel-2)] px-3.5 py-2 font-mono text-[13px] text-[var(--text)] outline-none focus:border-[var(--accent)]"
        />
        <button
          onClick={onSubmit}
          disabled={busy || !policy}
          className="rounded-lg bg-[var(--accent)] px-5 py-2 text-sm font-medium text-[#04122b] disabled:opacity-50"
        >
          {busy ? "gating…" : "Decide"}
        </button>
      </div>
      <div className="mt-2.5 flex flex-wrap items-center gap-1.5">
        {EXAMPLES.map((ex) => (
          <button
            key={ex}
            onClick={() => setValue(ex)}
            className="rounded-md border border-[var(--border)] px-2 py-0.5 font-mono text-[11px] text-[var(--muted)] hover:border-[var(--border-strong)] hover:text-[var(--text)]"
          >
            {ex}
          </button>
        ))}
      </div>
      {err && <div className="mt-2.5 text-[12px] text-[var(--deny)]">⚠ {err}</div>}
    </div>
  );
}

/* ------------------------------- stats ------------------------------ */

function Stats({ decisions, log }: { decisions: Decision[]; log: LogView | null }) {
  const total = decisions.length;
  const allowed = decisions.filter((d) => d.verdict === "allow").length;
  const blocked = decisions.filter((d) => d.verdict !== "allow").length;
  const pct = total ? Math.round((allowed / total) * 1000) / 10 : 0;
  return (
    <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
      <Stat label="Calls gated · session" value={String(total)} hint="this session" />
      <Stat label="Allowed" value={total ? `${pct}%` : "—"} tone="var(--allow)" hint={`${allowed} forwarded`} />
      <Stat label="Denied / escalated" value={String(blocked)} tone="var(--deny)" hint="never forwarded" />
      <Stat
        label="Recorded decisions"
        value={log ? String(log.verify.count) : "—"}
        hint={log?.verify.signed ? "checkpoint signed" : "hash-chained"}
      />
    </div>
  );
}

function Stat({ label, value, tone, hint }: { label: string; value: string; tone?: string; hint: string }) {
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--panel)] p-4">
      <div className="text-xs text-[var(--muted)]">{label}</div>
      <div className="mt-1.5 text-2xl font-semibold tracking-tight" style={tone ? { color: tone } : undefined}>
        {value}
      </div>
      <div className="mt-1 text-xs text-[var(--faint)]">{hint}</div>
    </div>
  );
}

/* ------------------------------- feed ------------------------------- */

function Feed({
  decisions,
  selected,
  onSelect,
}: {
  decisions: Decision[];
  selected: number;
  onSelect: (i: number) => void;
}) {
  return (
    <div className="overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--panel)]">
      <div className="flex items-center justify-between border-b border-[var(--border)] px-5 py-3.5">
        <div className="flex items-center gap-2.5">
          <span className="text-sm font-semibold">Decision stream</span>
          <span className="rounded-full bg-[var(--panel-3)] px-2 py-0.5 font-mono text-[11px] text-[var(--muted)]">
            {decisions.length}
          </span>
        </div>
      </div>

      {decisions.length === 0 ? (
        <div className="px-5 py-12 text-center text-sm text-[var(--faint)]">
          Propose a tool call above to see it gated.
        </div>
      ) : (
        <>
          <div className="grid grid-cols-[auto_1fr_auto_auto] gap-x-4 px-5 py-2 text-[11px] uppercase tracking-wider text-[var(--faint)]">
            <div>Time</div>
            <div>Tool · argument</div>
            <div className="text-right">Effect</div>
            <div className="text-right">Verdict</div>
          </div>
          <div>
            {decisions.map((d, i) => (
              <div
                key={i}
                onClick={() => onSelect(i)}
                className={`grid cursor-pointer grid-cols-[auto_1fr_auto_auto] items-center gap-x-4 border-t border-[var(--border)] px-5 py-3 ${
                  i === selected ? "bg-[var(--accent-soft)]" : "hover:bg-white/[0.02]"
                }`}
              >
                <div className="font-mono text-[12px] text-[var(--faint)]">{d.t}</div>
                <div className="min-w-0">
                  <span className="text-[13px] text-[var(--muted)]">{d.tool}</span>
                  <span className="mx-2 text-[var(--faint)]">·</span>
                  <span className="truncate font-mono text-[13px] text-[var(--text)]">{d.value || "∅"}</span>
                </div>
                <div className="text-right">
                  <span
                    className="rounded px-1.5 py-0.5 text-[11px]"
                    style={{
                      color: d.forward ? "var(--allow)" : "var(--faint)",
                      backgroundColor: d.forward ? "var(--allow-soft)" : "transparent",
                    }}
                  >
                    {d.forward ? "forwarded" : "blocked"}
                  </span>
                </div>
                <div className="flex justify-end">
                  <Pill v={d.verdict} />
                </div>
              </div>
            ))}
          </div>
        </>
      )}
    </div>
  );
}

/* ------------------------------ detail ------------------------------ */

function reason(d: Decision): string {
  if (d.fault?.startsWith("no recipe")) {
    return `No policy governs ${d.tool}. Unknown tools are refused by default — the call was blocked and never reached a downstream server.`;
  }
  if (d.verdict === "allow") {
    return `Cleared. "${d.value}" satisfies ${d.ruleFired ?? "the policy"} at an authoritative sink, so the call was forwarded to the real tool and recorded as a signed crossing.`;
  }
  if (d.verdict === "escalate") {
    return `Escalated. "${d.value}" needs human review; the call was held and not forwarded.`;
  }
  return `Denied. "${d.value}" is outside the allowed set for ${d.tool}; the authoritative sink refused the crossing and the call never reached the tool.`;
}

function Detail({
  decision,
  log,
  onVerify,
}: {
  decision: Decision | null;
  log: LogView | null;
  onVerify: () => void;
}) {
  if (!decision) {
    return (
      <div className="rounded-xl border border-[var(--border)] bg-[var(--panel)] p-5 text-sm text-[var(--faint)]">
        Select a decision to inspect the provable loop and the signed record.
      </div>
    );
  }
  const d = decision;
  const soft = V[d.verdict].soft;
  return (
    <div className="flex flex-col gap-4">
      <div className="rounded-xl border border-[var(--border)] bg-[var(--panel)] p-5">
        <div className="flex items-center justify-between">
          <span className="text-xs uppercase tracking-wider text-[var(--faint)]">Decision</span>
          <Pill v={d.verdict} />
        </div>
        <div className="mt-2 font-mono text-lg text-[var(--text)]">
          {d.tool}(<span className="text-[var(--accent)]">{d.value || "∅"}</span>)
        </div>
        <div className="mt-1 text-sm text-[var(--muted)]">
          agent proposal · <span className="font-mono">{d.subjectClass}</span> · {d.t}
        </div>

        <div className="mt-5 border-t border-[var(--border)] pt-5">
          <div className="mb-3 text-xs uppercase tracking-wider text-[var(--faint)]">Provable loop</div>
          <Chain chain={d.chain} />
        </div>
      </div>

      <div className="rounded-xl border border-[var(--border)] bg-[var(--panel)] p-5">
        <div className="grid grid-cols-2 gap-x-5 gap-y-4">
          <Attr label="Subject class" value={d.subjectClass} mono valueColor="var(--deny)" />
          <Attr label="Forwarded" value={d.forward ? "yes" : "no"} valueColor={d.forward ? "var(--allow)" : "var(--deny)"} />
          <Attr label="Rule fired" value={d.ruleFired || "—"} mono />
          <Attr label="Governed tool" value={d.tool} mono />
        </div>
        <div
          className="mt-5 rounded-lg px-3.5 py-3 text-[13px] leading-relaxed text-[var(--muted)]"
          style={{ backgroundColor: soft, border: `1px solid ${soft}` }}
        >
          <span className="font-medium text-[var(--text)]">{V[d.verdict].label[0] + V[d.verdict].label.slice(1).toLowerCase()}.</span>{" "}
          {reason(d).replace(/^[A-Za-z]+\.\s/, "")}
        </div>
      </div>

      <div className="rounded-xl border border-[var(--border)] bg-[var(--panel)] p-5">
        <div className="mb-3 flex items-center justify-between">
          <span className="text-xs uppercase tracking-wider text-[var(--faint)]">Signed record</span>
          {log && (
            <span
              className="flex items-center gap-1.5 text-xs"
              style={{ color: log.verify.error ? "var(--deny)" : "var(--allow)" }}
            >
              <CheckIcon /> {log.verify.error ? "tampered" : log.verify.signed ? (log.verify.verified ? "verified" : "signed") : "hash-chained"}
            </span>
          )}
        </div>
        <Row k="Audit log" v={log ? `${log.verify.count} decisions` : "…"} />
        <Row k="Head" v={log?.verify.head ? `sha256:${log.verify.head.slice(0, 12)}…` : "—"} mono />
        {log?.verify.signed && <Row k="Signed by" v={log.verify.keyId || "—"} mono />}
        <div className="mt-4 flex gap-2.5">
          <button
            onClick={onVerify}
            className="flex-1 rounded-lg bg-[var(--accent)] py-2 text-sm font-medium text-[#04122b]"
          >
            Verify chain
          </button>
        </div>
      </div>
    </div>
  );
}

/* ------------------------------- bits ------------------------------- */

function chainColor(state: string): string {
  if (state === "allow" || state === "ok") return "var(--allow)";
  if (state === "deny") return "var(--deny)";
  if (state === "escalate") return "var(--escalate)";
  return "var(--faint)"; // skip
}

function Chain({ chain }: { chain: DecisionView["chain"] }) {
  const nodes: [string, string][] = [
    ["sense", chain.sense],
    ["reason", chain.reason],
    ["decide", chain.decide],
    ["act", chain.act],
    ["prove", chain.prove],
  ];
  return (
    <div className="flex items-start justify-between">
      {nodes.map(([label, state], i) => {
        const c = chainColor(state);
        const skip = state === "skip";
        const bad = state === "deny" || state === "escalate";
        return (
          <div key={label} className="relative flex flex-1 flex-col items-center">
            {i < nodes.length - 1 && (
              <span className="absolute left-1/2 top-[13px] h-px w-full bg-[var(--border-strong)]" />
            )}
            <span
              className="relative z-10 flex h-[26px] w-[26px] items-center justify-center rounded-full border-2"
              style={{ borderColor: c, background: "var(--panel)", color: c, opacity: skip ? 0.5 : 1 }}
            >
              {bad ? <XMini /> : skip ? <DashMini /> : <CheckMini />}
            </span>
            <span
              className="mt-2 text-[11px]"
              style={{ color: skip ? "var(--faint)" : "var(--muted)", opacity: skip ? 0.7 : 1 }}
            >
              {label}
            </span>
          </div>
        );
      })}
    </div>
  );
}

function Attr({ label, value, mono, valueColor }: { label: string; value: string; mono?: boolean; valueColor?: string }) {
  return (
    <div>
      <div className="text-xs text-[var(--faint)]">{label}</div>
      <div className={`mt-0.5 text-sm ${mono ? "font-mono" : ""}`} style={{ color: valueColor ?? "var(--text)" }}>
        {value}
      </div>
    </div>
  );
}

function Row({ k, v, mono }: { k: string; v: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between border-t border-[var(--border)] py-2.5 first:border-t-0 first:pt-0">
      <span className="text-sm text-[var(--muted)]">{k}</span>
      <span className={`text-sm text-[var(--text)] ${mono ? "font-mono text-[13px]" : ""}`}>{v}</span>
    </div>
  );
}

function Pill({ v }: { v: Verdict }) {
  const s = V[v];
  return (
    <span
      className="rounded-md px-2 py-0.5 text-[11px] font-semibold tracking-wide"
      style={{ color: s.color, backgroundColor: s.soft }}
    >
      {s.label}
    </span>
  );
}

/* ------------------------------- icons ------------------------------ */

function CheckIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d="m5 12 4.5 4.5L19 7" />
    </svg>
  );
}
function CheckMini() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden>
      <path d="m5 12 4.5 4.5L19 7" />
    </svg>
  );
}
function XMini() {
  return (
    <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.6" strokeLinecap="round" aria-hidden>
      <path d="M6 6l12 12M18 6 6 18" />
    </svg>
  );
}
function DashMini() {
  return (
    <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.6" strokeLinecap="round" aria-hidden>
      <path d="M6 12h12" />
    </svg>
  );
}
