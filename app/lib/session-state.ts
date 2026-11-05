"use client";

import type { SessionResponse } from "./api/types";

export type SessionState = { session: SessionResponse | null; error: string | null };
const initial: SessionState = { session: null, error: null };
let state = initial;
const listeners = new Set<() => void>();

export function readSessionState() { return state; }
export function readServerSessionState() { return initial; }
export function subscribeSession(listener: () => void) {
  listeners.add(listener);
  return () => { listeners.delete(listener); };
}
export function publishSession(session: SessionResponse) {
  state = { session, error: null };
  listeners.forEach((listener) => listener());
}
export function publishSessionError(error: unknown) {
  state = { ...state, error: error instanceof Error ? error.message : "Your connection was interrupted." };
  listeners.forEach((listener) => listener());
}
