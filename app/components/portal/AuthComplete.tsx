"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { QRCodeSVG } from "qrcode.react";
import {
  beginTOTPEnrollment,
  clearSession,
  completeMFAChallenge,
  confirmTOTPEnrollment,
  getMFAStatus,
  getSession,
} from "@/app/lib/auth-client";
import { APIError } from "@/app/lib/api/client";
import type { SessionUser, TOTPEnrollment } from "@/app/lib/api/types";
import { authorizedReturnPath, safeReturnPath } from "@/app/lib/role-routing";

type MFAState =
  | { kind: "loading" }
  | { kind: "redirecting" }
  | { kind: "sign-in" }
  | { kind: "setup"; enrollmentPending: boolean; busy: boolean; error?: string }
  | { kind: "enrollment"; enrollment: TOTPEnrollment; busy: boolean; error?: string }
  | { kind: "challenge"; method: "totp" | "recovery"; busy: boolean; error?: string }
  | { kind: "recovery"; recoveryCodes: string[]; user: SessionUser; acknowledged: boolean }
  | { kind: "error"; message: string };

function expiredSignIn(error: unknown) {
  return error instanceof APIError && error.status === 401
    && ["mfa_challenge_expired", "unauthenticated"].includes(error.problem?.code || "");
}

function messageFor(error: unknown, fallback: string) {
  return error instanceof Error && error.message.trim() ? error.message : fallback;
}

function safeMFAReturnPath(value: string | null) {
  const path = safeReturnPath(value);
  if (
    path.includes("\\")
    || /[\u0000-\u001f\u007f]/.test(path)
    || path === "/auth"
    || path.startsWith("/auth/")
    || path === "/sign-in"
  ) {
    return "/portal";
  }
  return path;
}

export default function AuthComplete() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [state, setState] = useState<MFAState>({ kind: "loading" });

  const restartSignIn = useCallback(() => {
    clearSession();
    setState({ kind: "sign-in" });
    router.replace(`/sign-in?error=mfa_challenge_expired&next=${encodeURIComponent(safeMFAReturnPath(searchParams.get("next")))}`);
  }, [router, searchParams]);

  function destinationFor(user: SessionUser) {
    const requested = safeMFAReturnPath(searchParams.get("next"));
    return authorizedReturnPath(user, requested);
  }

  function redirect(user: SessionUser) {
    setState({ kind: "redirecting" });
    router.replace(destinationFor(user));
  }

  useEffect(() => {
    let active = true;

    async function completePrimarySignIn() {
      try {
        const session = await getSession();
        if (!active) return;
        if (session.authenticated && session.user) {
          setState({ kind: "redirecting" });
          const requested = safeMFAReturnPath(searchParams.get("next"));
          router.replace(authorizedReturnPath(session.user, requested));
          return;
        }

        const mfaRequested = searchParams.get("mfa") === "required";
        if (!mfaRequested && !session.mfaRequired) {
          restartSignIn();
          return;
        }

        const status = await getMFAStatus();
        if (!active) return;
        if (!status.required) {
          setState({
            kind: "error",
            message: "This account does not have a multi-factor challenge to complete.",
          });
          return;
        }
        if (status.verifiedThisSession) {
          const verifiedSession = await getSession();
          if (!active) return;
          if (verifiedSession.authenticated && verifiedSession.user) {
            setState({ kind: "redirecting" });
            const requested = safeMFAReturnPath(searchParams.get("next"));
            router.replace(authorizedReturnPath(verifiedSession.user, requested));
            return;
          }
          throw new Error("The verified session could not be opened.");
        }

        setState(status.enrolled
          ? { kind: "challenge", method: "totp", busy: false }
          : { kind: "setup", enrollmentPending: status.enrollmentPending, busy: false });
      } catch (error) {
        if (active) {
          if (expiredSignIn(error)) { restartSignIn(); return; }
          setState({
            kind: "error",
            message: messageFor(error, "We could not verify this sign-in attempt."),
          });
        }
      }
    }

    void completePrimarySignIn();
    return () => {
      active = false;
    };
  }, [router, searchParams, restartSignIn]);

  async function startEnrollment() {
    if (state.kind !== "setup" || state.busy) return;
    setState({ ...state, busy: true, error: undefined });
    try {
      const enrollment = await beginTOTPEnrollment();
      setState({ kind: "enrollment", enrollment, busy: false });
    } catch (error) {
      if (expiredSignIn(error)) { restartSignIn(); return; }
      setState({
        ...state,
        busy: false,
        error: messageFor(error, "We could not start authenticator setup."),
      });
    }
  }

  async function submitEnrollment(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (state.kind !== "enrollment" || state.busy) return;
    const code = String(new FormData(event.currentTarget).get("code") || "").trim();
    if (!/^\d{6}$/.test(code)) {
      setState({ ...state, error: "Enter the six-digit code from your authenticator app." });
      return;
    }

    setState({ ...state, busy: true, error: undefined });
    try {
      const result = await confirmTOTPEnrollment(code);
      if (!result.authenticated || !result.user || result.recoveryCodes.length === 0) {
        throw new Error("Authenticator setup completed without the required recovery codes.");
      }
      setState({
        kind: "recovery",
        recoveryCodes: result.recoveryCodes,
        user: result.user,
        acknowledged: false,
      });
    } catch (error) {
      if (expiredSignIn(error)) { restartSignIn(); return; }
      setState({
        ...state,
        busy: false,
        error: messageFor(error, "We could not confirm that authenticator code."),
      });
    }
  }

  async function submitChallenge(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (state.kind !== "challenge" || state.busy) return;
    const credential = String(new FormData(event.currentTarget).get("credential") || "").trim();
    if (state.method === "totp" && !/^\d{6}$/.test(credential)) {
      setState({ ...state, error: "Enter the six-digit code from your authenticator app." });
      return;
    }
    if (state.method === "recovery" && !credential) {
      setState({ ...state, error: "Enter one of your saved recovery codes." });
      return;
    }

    setState({ ...state, busy: true, error: undefined });
    try {
      const session = await completeMFAChallenge(
        state.method === "totp" ? { code: credential } : { recoveryCode: credential },
      );
      if (!session.authenticated || !session.user) {
        throw new Error("The multi-factor challenge did not open an authenticated session.");
      }
      redirect(session.user);
    } catch (error) {
      if (expiredSignIn(error)) { restartSignIn(); return; }
      setState({
        ...state,
        busy: false,
        error: messageFor(error, "We could not verify that credential."),
      });
    }
  }

  if (state.kind === "error") {
    return (
      <div className="auth-success" role="alert">
        <span>Sign-in not completed</span>
        <h1>We could not open your academy portal.</h1>
        <p>{state.message}</p>
        <Link href={`/sign-in?next=${encodeURIComponent(safeMFAReturnPath(searchParams.get("next")))}`} className="button button-gold">Sign in again</Link>
      </div>
    );
  }

  if (state.kind === "setup") {
    return (
      <div className="auth-form">
        <span className="eyebrow">Required security setup</span>
        <h1>Protect your staff account with an authenticator.</h1>
        <p>Coaches and staff must add a time-based authenticator before entering the academy portal.</p>
        {state.enrollmentPending && (
          <p className="mfa-security-note">A previous setup was not completed. Starting again replaces that unused setup secret.</p>
        )}
        {state.error && <p className="form-message form-message-error" role="alert">{state.error}</p>}
        <button className="button button-gold button-wide" disabled={state.busy} onClick={startEnrollment}>
          {state.busy ? "Preparing authenticator…" : "Set up authenticator"}
        </button>
        <Link href="/sign-in" className="text-link">Back to sign in</Link>
      </div>
    );
  }

  if (state.kind === "enrollment") {
    return (
      <form className="auth-form" onSubmit={submitEnrollment}>
        <span className="eyebrow">Authenticator setup</span>
        <h1>Add KingsGambit2700 to your authenticator.</h1>
        <p>Scan this QR code with your authenticator app, then enter the six-digit code it shows.</p>
        <figure className="mfa-enrollment-qr">
          <QRCodeSVG
            value={state.enrollment.otpauthUri}
            size={240}
            level="M"
            marginSize={4}
            bgColor="#ffffff"
            fgColor="#000000"
            role="img"
            aria-label="Authenticator setup QR code"
          />
          <figcaption>In your authenticator app, choose Add account, then Scan a QR code.</figcaption>
        </figure>
        <div className="mfa-enrollment-details">
          <div>
            <span>Can’t scan? Enter this setup key manually</span>
            <code aria-label="Authenticator setup secret">{state.enrollment.secret}</code>
            <a className="text-link" href={state.enrollment.otpauthUri}>Open in authenticator app</a>
          </div>
        </div>
        <label>
          <span>Six-digit authenticator code</span>
          <input
            name="code"
            className="code-input"
            inputMode="numeric"
            autoComplete="one-time-code"
            pattern="[0-9]{6}"
            minLength={6}
            maxLength={6}
            required
          />
        </label>
        {state.error && <p className="form-message form-message-error" role="alert">{state.error}</p>}
        <button className="button button-gold button-wide" disabled={state.busy}>
          {state.busy ? "Confirming code…" : "Confirm authenticator"}
        </button>
      </form>
    );
  }

  if (state.kind === "challenge") {
    const usingTOTP = state.method === "totp";
    return (
      <form className="auth-form" onSubmit={submitChallenge}>
        <span className="eyebrow">Multi-factor verification</span>
        <h1>Complete your secure sign-in.</h1>
        <p>Use your authenticator code. If your device is unavailable, use one saved recovery code instead.</p>
        <fieldset className="mfa-method-choice">
          <legend>Verification method</legend>
          <label>
            <input
              type="radio"
              name="method"
              value="totp"
              checked={usingTOTP}
              onChange={() => setState({ kind: "challenge", method: "totp", busy: false })}
            />
            Authenticator code
          </label>
          <label>
            <input
              type="radio"
              name="method"
              value="recovery"
              checked={!usingTOTP}
              onChange={() => setState({ kind: "challenge", method: "recovery", busy: false })}
            />
            Recovery code
          </label>
        </fieldset>
        <label>
          <span>{usingTOTP ? "Six-digit authenticator code" : "Recovery code"}</span>
          <input
            key={state.method}
            name="credential"
            className={usingTOTP ? "code-input" : undefined}
            inputMode={usingTOTP ? "numeric" : "text"}
            autoComplete={usingTOTP ? "one-time-code" : "off"}
            pattern={usingTOTP ? "[0-9]{6}" : undefined}
            minLength={usingTOTP ? 6 : undefined}
            maxLength={usingTOTP ? 6 : 40}
            spellCheck={false}
            required
            autoFocus
          />
        </label>
        {state.error && <p className="form-message form-message-error" role="alert">{state.error}</p>}
        <button className="button button-gold button-wide" disabled={state.busy}>
          {state.busy ? "Verifying…" : "Verify and continue"}
        </button>
      </form>
    );
  }

  if (state.kind === "recovery") {
    return (
      <div className="auth-form" role="region" aria-labelledby="mfa-recovery-heading">
        <span className="eyebrow">Authenticator enabled</span>
        <h1 id="mfa-recovery-heading">Save your recovery codes now.</h1>
        <p>Each code works once if you lose access to your authenticator. These codes will not be shown again.</p>
        <ol className="mfa-recovery-codes" aria-label="One-time recovery codes">
          {state.recoveryCodes.map((code) => <li key={code}><code>{code}</code></li>)}
        </ol>
        <label className="check-field mfa-save-confirmation">
          <input
            type="checkbox"
            checked={state.acknowledged}
            onChange={(event) => setState({ ...state, acknowledged: event.target.checked })}
          />
          <span>I saved all recovery codes in a secure place.</span>
        </label>
        <button
          className="button button-gold button-wide"
          disabled={!state.acknowledged}
          onClick={() => redirect(state.user)}
        >
          Continue to academy portal
        </button>
      </div>
    );
  }

  return (
    <div className="auth-success" aria-busy="true" aria-live="polite">
      <div className="portal-loader" aria-hidden="true" />
      <h1>{state.kind === "sign-in" ? "Returning to sign-in…" : state.kind === "redirecting" ? "Opening your academy portal…" : "Completing secure sign-in…"}</h1>
    </div>
  );
}
