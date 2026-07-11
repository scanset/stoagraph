"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  dispatch,
  getEventMap,
  listModels,
  saveEventMap,
  type AgentEvent,
  type Definition,
  type ModelView,
} from "../lib/harness";

/* The turnkey path, watched live:
 *
 *   EVENT -> dispatcher picks the recipe -> session bound on the gate -> governed agent runs
 *
 * The model never chooses its own recipe: the dispatcher binds the session server-side, and stag
 * enforces whatever recipe that session was bound to. A misroute cannot breach — it can only fail.
 * Every proposal in the transcript below crossed the gate before it did anything. */

const SAMPLE = `{
  "source": "pagerduty",
  "event": { "type": "incident.triggered" },
  "title": "prod web returning 500s",
  "detail": "The web deployment in prod is throwing HTTP 500s to live customers. Investigate, then remediate."
}`;

export default function DispatchPage() {
  const [models, setModels] = useState<ModelView[]>([]);
  const [model, setModel] = useState("");
  const [dispatchModel, setDispatchModel] = useState("");
  const [eventSrc, setEventSrc] = useState(SAMPLE);
  const [events, setEvents] = useState<AgentEvent[]>([]);
  const [running, setRunning] = useState(false);
  const [msg, setMsg] = useState<string | null>(null);
  const abortRef = useRef<(() => void) | null>(null);
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    listModels()
      .then((m) => {
        setModels(m);
        if (m.length && !model) setModel(m[0].name);
      })
      .catch((e) => setMsg(e instanceof Error ? e.message : String(e)));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [events]);

  const run = useCallback(() => {
    setMsg(null);
    let event: unknown;
    try {
      event = JSON.parse(eventSrc);
    } catch {
      setMsg("the event must be valid JSON");
      return;
    }
    setEvents([]);
    setRunning(true);
    const { done, abort } = dispatch(
      { event, model, dispatchModel: dispatchModel || undefined, maxTurns: 8 },
      (e) => setEvents((prev) => [...prev, e]),
    );
    abortRef.current = abort;
    done
      .catch((e) => setMsg(e instanceof Error ? e.message : String(e)))
      .finally(() => {
        setRunning(false);
        abortRef.current = null;
      });
  }, [eventSrc, model, dispatchModel]);

  return (
    <>
      <header className="flex h-14 shrink-0 items-center justify-between border-b border-[var(--border)] px-6">
        <div className="flex items-baseline gap-3">
          <h1 className="text-[15px] font-semibold tracking-tight">Dispatch</h1>
          <span className="text-sm text-[var(--faint)]">event → recipe → governed agent</span>
        </div>
        {msg && <span className="text-[12px] text-[var(--deny)]">⚠ {msg}</span>}
      </header>

      <div className="flex-1 overflow-auto p-6">
        <div className="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,420px)_minmax(0,1fr)]">
          <div className="flex flex-col gap-6">
            <Card title="Dispatch an event" sub="the dispatcher routes it; the gate governs it">
              <div className="flex flex-col gap-3 p-4">
                <div className="flex gap-2">
                  <Field label="proposer model">
                    <Select value={model} onChange={setModel} options={models.map((m) => m.name)} />
                  </Field>
                  <Field label="dispatch model (optional)">
                    <Select
                      value={dispatchModel}
                      onChange={setDispatchModel}
                      options={models.map((m) => m.name)}
                      empty="— deterministic map only —"
                    />
                  </Field>
                </div>
                <label className="flex flex-col gap-1 text-[11px] font-medium text-[var(--muted)]">
                  event (JSON)
                  <textarea
                    value={eventSrc}
                    onChange={(e) => setEventSrc(e.target.value)}
                    rows={9}
                    spellCheck={false}
                    className="rounded-md border border-[var(--border-strong)] bg-[var(--panel-2)] px-2.5 py-2 font-mono text-[12px] text-[var(--text)] outline-none focus:border-[var(--accent)]"
                  />
                </label>
                <div className="flex gap-2">
                  <button
                    onClick={run}
                    disabled={running || !model}
                    className="rounded-md bg-[var(--accent)] px-3.5 py-1.5 text-[12px] font-medium text-[#04122b] disabled:opacity-40"
                  >
                    {running ? "running…" : "Dispatch"}
                  </button>
                  {running && (
                    <button
                      onClick={() => abortRef.current?.()}
                      className="rounded-md border border-[var(--border-strong)] px-3.5 py-1.5 text-[12px] text-[var(--muted)]"
                    >
                      Stop
                    </button>
                  )}
                </div>
              </div>
            </Card>

            <EventMapEditor />
          </div>

          <Card title="Transcript" sub="dispatcher routes · model proposes · the gate disposes">
            <div className="flex max-h-[70vh] min-h-[340px] flex-col gap-2 overflow-auto p-4">
              {events.length === 0 && (
                <div className="py-16 text-center text-[12px] text-[var(--faint)]">
                  Dispatch an event to watch it route to a recipe and run the gated agent.
                </div>
              )}
              {events.map((e, i) => (
                <EventRow key={i} e={e} />
              ))}
              <div ref={endRef} />
            </div>
          </Card>
        </div>
      </div>
    </>
  );
}

/* A gate verdict is the load-bearing line in this transcript: `allowed` false means the call was
 * NEVER forwarded downstream. We colour by that, not by the model's confidence. */
function EventRow({ e }: { e: AgentEvent }) {
  const text = e.text ?? e.result ?? "";
  const tone =
    e.kind === "error" || (e.kind === "verdict" && e.allowed === false)
      ? "var(--deny)"
      : e.kind === "verdict" || e.kind === "retry"
        ? "var(--allow)"
        : e.kind === "await"
          ? "var(--escalate)"
          : e.kind === "dispatch"
            ? "var(--accent)"
            : "var(--border)";
  return (
    <div className="rounded-lg border px-3 py-2" style={{ borderColor: tone }}>
      <div className="flex items-center gap-2">
        <span className="text-[10px] uppercase tracking-wider text-[var(--faint)]">{e.kind}</span>
        {e.tool && <span className="font-mono text-[11px] text-[var(--muted)]">{e.tool}</span>}
      </div>
      {text && (
        <pre className="mt-1 whitespace-pre-wrap break-words font-mono text-[12px] text-[var(--muted)]">
          {text.length > 1200 ? `${text.slice(0, 1200)}…` : text}
        </pre>
      )}
    </div>
  );
}

/* The event map is the DETERMINISTIC layer: a user-authored quick reference the dispatcher checks
 * first. The dispatch model is the fallback for events that don't fit cleanly — it never gets to
 * widen what a session may do, because the recipe still governs. */
function EventMapEditor() {
  const [src, setSrc] = useState("");
  const [note, setNote] = useState<string | null>(null);

  useEffect(() => {
    getEventMap()
      .then((d) => setSrc(JSON.stringify(d, null, 2)))
      .catch(() => setSrc("[]"));
  }, []);

  const save = async () => {
    setNote(null);
    let defs: Definition[];
    try {
      defs = JSON.parse(src);
    } catch {
      setNote("not valid JSON");
      return;
    }
    try {
      const res = await saveEventMap(defs);
      setNote(res.ok ? `saved — ${res.count ?? defs.length} definition(s)` : (res.error ?? "save failed"));
    } catch (e) {
      setNote(e instanceof Error ? e.message : String(e));
    }
  };

  return (
    <Card title="Event map" sub="deterministic first; the dispatch model is the fallback">
      <div className="flex flex-col gap-3 p-4">
        <textarea
          value={src}
          onChange={(e) => setSrc(e.target.value)}
          rows={12}
          spellCheck={false}
          className="rounded-md border border-[var(--border-strong)] bg-[var(--panel-2)] px-2.5 py-2 font-mono text-[11px] text-[var(--text)] outline-none focus:border-[var(--accent)]"
        />
        <div className="flex items-center gap-3">
          <button
            onClick={save}
            className="rounded-md border border-[var(--border-strong)] px-3.5 py-1.5 text-[12px] text-[var(--muted)] hover:border-[var(--accent)] hover:text-[var(--text)]"
          >
            Save map
          </button>
          {note && <span className="text-[11px] text-[var(--faint)]">{note}</span>}
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

function Select({
  value,
  onChange,
  options,
  empty,
}: {
  value: string;
  onChange: (v: string) => void;
  options: string[];
  empty?: string;
}) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="min-w-0 flex-1 rounded-md border border-[var(--border-strong)] bg-[var(--panel-2)] px-2.5 py-1.5 font-mono text-[12px] text-[var(--text)] outline-none focus:border-[var(--accent)]"
    >
      {empty && <option value="">{empty}</option>}
      {options.map((o) => (
        <option key={o} value={o}>
          {o}
        </option>
      ))}
    </select>
  );
}
