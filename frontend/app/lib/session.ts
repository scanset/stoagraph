"use client";

// One login, two role-scoped keys — and the human never sees the seam.
//
// The console talks to two backends: the GATE (policy + approvals) and the ORCHESTRATOR (models +
// dispatch). Those need DIFFERENT secrets, and that is not incidental: the orchestrator must hold its
// own secret to verify the human, and if that secret could also approve, a compromised orchestrator
// could approve its own escalations. So there are two keys. But you log in once.
//
// `stoagraph up` prints a link like  http://localhost:3000/#c=<console>&o=<operator>
// The keys live in the URL FRAGMENT (#...), which a browser never sends to any server. We read them
// once, store them in localStorage, and strip them from the address bar. No copy-pasting a raw token.

import { setToken } from "./api";
import { setOperatorToken } from "./harness";

const CONSOLE_KEY = "stag.control.token"; // must match api.ts
const OPERATOR_KEY = "stoagraph.operator.token"; // must match harness.ts

/** adoptLoginFromURL reads #c/#o from the fragment, stores them, and clears the fragment. Returns true
 *  if it just logged the user in (so the UI can refresh). Safe to call on every mount. */
export function adoptLoginFromURL(): boolean {
  if (typeof window === "undefined") return false;
  const hash = window.location.hash.replace(/^#/, "");
  if (!hash) return false;
  const p = new URLSearchParams(hash);
  const c = p.get("c");
  const o = p.get("o");
  if (!c && !o) return false;

  if (c) setToken(c);
  if (o) setOperatorToken(o);

  // Remove the secrets from the address bar and browser history immediately.
  history.replaceState(null, "", window.location.pathname + window.location.search);
  return true;
}

export function isLoggedIn(): boolean {
  if (typeof window === "undefined") return false;
  return !!window.localStorage.getItem(CONSOLE_KEY);
}

export function signOut(): void {
  if (typeof window === "undefined") return;
  window.localStorage.removeItem(CONSOLE_KEY);
  window.localStorage.removeItem(OPERATOR_KEY);
}
