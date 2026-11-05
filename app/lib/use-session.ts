"use client";

import { useEffect, useSyncExternalStore } from "react";
import { clearSession, getSession } from "./auth-client";
import { readServerSessionState, readSessionState, subscribeSession } from "./session-state";
import { isDemoMode } from "./demo/mode";

export function useSession() {
  const state = useSyncExternalStore(subscribeSession, readSessionState, readServerSessionState);
  useEffect(() => {
    const refresh = () => { void getSession().catch(() => undefined); };
    refresh();
    window.addEventListener("focus", refresh);
    const channel = !isDemoMode() && typeof BroadcastChannel !== "undefined" ? new BroadcastChannel("academy-session") : null;
    if (channel) channel.onmessage = (event) => {
      if (event.data === "signed-out") clearSession();
      else refresh();
    };
    return () => { window.removeEventListener("focus", refresh); channel?.close(); };
  }, []);
  return state;
}
