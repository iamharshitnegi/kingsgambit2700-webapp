"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { apiFetch } from "@/app/lib/api/client";
import { getSession, logout } from "@/app/lib/auth-client";
import { Panel, Empty, time } from "./WorkspaceUI";
import "./device-sessions.css";

type DeviceSession = {
  id: string;
  createdAt: string;
  lastSeenAt: string;
  current: boolean;
  remembered: boolean;
};

export default function DeviceSessions() {
  const router = useRouter();
  const [devices, setDevices] = useState<DeviceSession[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  const load = useCallback(async () => {
    const session = await getSession();
    if (!session.authenticated) throw new Error("Sign in to manage your browsers.");
    return (await apiFetch<{ items: DeviceSession[] }>("/auth/sessions")).items;
  }, []);

  useEffect(() => {
    let active = true;
    void load().then((items) => { if (active) setDevices(items); }).catch((failure: unknown) => {
      if (active) setError(failure instanceof Error ? failure.message : "We could not load your browsers.");
    });
    return () => { active = false; };
  }, [load]);

  async function revoke(device?: DeviceSession) {
    setBusy(true);
    setError("");
    setMessage("");
    try {
      if (device?.current) {
        await logout();
        router.replace("/sign-in");
        return;
      }
      const session = await getSession();
      if (!session.authenticated) throw new Error("Sign in to manage your browsers.");
      await apiFetch(device ? `/auth/sessions/${encodeURIComponent(device.id)}` : "/auth/sessions/revoke-others", {
        method: device ? "DELETE" : "POST",
      });
      setDevices(await load());
      setMessage(device ? "That browser has been signed out." : "Your other browsers have been signed out.");
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "We could not sign out that browser. Please try again.");
    } finally {
      setBusy(false);
    }
  }

  async function retry() {
    setBusy(true);
    setError("");
    try { setDevices(await load()); }
    catch (failure) { setError(failure instanceof Error ? failure.message : "We could not load your browsers."); }
    finally { setBusy(false); }
  }

  return <Panel title="Remembered browsers" action={devices?.some((device) => !device.current)
    ? <button type="button" className="button button-quiet" disabled={busy} onClick={() => void revoke()}>Sign out other browsers</button>
    : undefined}>
    <p>Your academy stays signed in on these browsers until you sign out or remove their access.</p>
    {error && <div role="alert"><p>{error}</p><button type="button" className="text-link" disabled={busy} onClick={() => void retry()}>Try again</button></div>}
    {message && <p role="status">{message}</p>}
    {!devices && !error && <p role="status">Loading your browsers…</p>}
    {devices?.length === 0 && <Empty>No remembered browsers to show.</Empty>}
    {devices && devices.length > 0 && <ul className="device-session-list">{devices.map((device, index) => <li key={device.id}>
      <div><strong>{device.current ? "This browser" : `Browser ${index + 1}`}</strong>
        <p>Signed in {time(device.createdAt)} · Last active {time(device.lastSeenAt)}</p>
      </div>
      <button type="button" className="button button-quiet" disabled={busy} onClick={() => void revoke(device)}>
        {device.current ? "Sign out this browser" : `Sign out browser ${index + 1}`}
      </button>
    </li>)}</ul>}
  </Panel>;
}
