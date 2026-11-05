"use client";

import Script from "next/script";
import { useEffect, useRef, useState } from "react";
import { getGoogleConfiguration, signInWithGoogle } from "@/app/lib/auth-client";
import type { SessionResponse } from "@/app/lib/api/types";

type GoogleIdentityServices = {
  accounts: { id: {
    initialize: (options: { client_id: string; nonce: string; callback: (response: { credential: string }) => void; auto_select: boolean }) => void;
    renderButton: (container: HTMLElement, options: { type: string; theme: string; size: string; text: string; width: number }) => void;
  } };
};

export default function GoogleSignIn({ onSuccess }: { onSuccess: (session: SessionResponse) => void }) {
  const container = useRef<HTMLDivElement>(null);
  const success = useRef(onSuccess);
  const initialConfiguration = useRef<ReturnType<typeof getGoogleConfiguration> | null>(null);
  const [configuration, setConfiguration] = useState<Awaited<ReturnType<typeof getGoogleConfiguration>> | null>(null);
  const [ready, setReady] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => { success.current = onSuccess; }, [onSuccess]);
  useEffect(() => {
    let active = true;
    // Strict Mode repeats effects; share the request so its cookie matches its nonce.
    initialConfiguration.current ??= getGoogleConfiguration();
    initialConfiguration.current.then((value) => { if (active) setConfiguration(value); })
      .catch(() => { if (active) setError("Google sign-in is unavailable. You can still sign in with your password."); });
    return () => { active = false; };
  }, []);

  useEffect(() => {
    if (!configuration?.enabled) return;
    let active = true;
    const timer = setTimeout(() => {
      getGoogleConfiguration().then((value) => { if (active) setConfiguration(value); })
        .catch(() => { if (active) setError("Refresh the page to try Google sign-in again."); });
    }, 8 * 60 * 1000);
    return () => { active = false; clearTimeout(timer); };
  }, [configuration]);

  useEffect(() => {
    const google = (window as unknown as { google?: GoogleIdentityServices }).google;
    if (!ready || !google || !container.current || !configuration?.clientId || !configuration.nonce) return;
    google.accounts.id.initialize({
      client_id: configuration.clientId,
      nonce: configuration.nonce,
      auto_select: false,
      callback: async ({ credential }) => {
        setBusy(true);
        setError("");
        try { success.current(await signInWithGoogle(credential)); }
        catch (failure) {
          setError(failure instanceof Error ? failure.message : "Google sign-in failed. Please try again.");
          // Renew a challenge invalidated by expiry or another sign-in tab.
          try { setConfiguration(await getGoogleConfiguration()); }
          catch { setError("Refresh the page to try Google sign-in again."); }
        }
        finally { setBusy(false); }
      },
    });
    google.accounts.id.renderButton(container.current, { type: "standard", theme: "outline", size: "large", text: "continue_with", width: Math.min(360, container.current.clientWidth || 280) });
  }, [ready, configuration]);

  if (configuration && !configuration.enabled) return null;
  return <div className="auth-google" aria-busy={busy}>
    {configuration?.enabled && <Script src="https://accounts.google.com/gsi/client" strategy="afterInteractive" onReady={() => setReady(true)} onError={() => setError("Google could not load. Please use your password or try again.")} />}
    <div ref={container} />
    {busy && <p role="status">Signing in with Google…</p>}
    {error && <p className="form-message form-message-error" role="alert">{error}</p>}
  </div>;
}
