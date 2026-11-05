"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";
import { registerWithPassword, signInWithPassword } from "@/app/lib/auth-client";
import type { SessionResponse } from "@/app/lib/api/types";
import { authorizedReturnPath, safeReturnPath } from "@/app/lib/role-routing";
import { useSession } from "@/app/lib/use-session";
import { isDemoMode } from "@/app/lib/demo/mode";
import GoogleSignIn from "./GoogleSignIn";
import MagicLinkForm from "./MagicLinkForm";

export default function SignInForm() {
  const router = useRouter();
  const search = useSearchParams();
  const [mode, setMode] = useState<"login" | "register" | "email">("login");
  const [busy, setBusy] = useState(false);
  const [sent, setSent] = useState(false);
  const [error, setError] = useState("");
  const next = safeReturnPath(search.get("next"));
  const queryError = search.get("error");
  const { session, error: sessionError } = useSession();
  useEffect(() => {
    if (session?.authenticated && session.user) router.replace(authorizedReturnPath(session.user, next));
    else if (session?.mfaRequired) router.replace(`/auth/complete?mfa=required&next=${encodeURIComponent(next)}`);
  }, [session, next, router]);
  const linkError = queryError === "mfa_challenge_expired"
    ? "Your sign-in verification expired. Please sign in again."
    : queryError === "registration_conflict"
    ? "That email or username is already in use. Sign in to your existing account, or register with another username."
    : queryError === "invalid_or_expired_link" ? "This link has expired or has already been used. Request a new email link." : "";

  function complete(session: SessionResponse) {
    router.replace(`/auth/complete?${session.mfaRequired ? "mfa=required&" : ""}next=${encodeURIComponent(next)}`);
  }

  function changeMode(value: typeof mode) {
    setMode(value);
    setError("");
    setSent(false);
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    setBusy(true);
    setError("");
    try {
      if (mode === "register") {
        await registerWithPassword({ email: String(data.get("email") || "").trim(),
          username: String(data.get("username") || "").trim(), displayName: String(data.get("displayName") || "").trim(),
          password: String(data.get("password") || ""), redirectPath: `/auth/complete?next=${encodeURIComponent(next)}` });
        setSent(true);
      } else {
        complete(await signInWithPassword(String(data.get("login") || "").trim(), String(data.get("password") || "")));
      }
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : "We could not sign you in. Please try again.");
    } finally { setBusy(false); }
  }

  if (isDemoMode()) return <MagicLinkForm />;
  if (!session && !sessionError) return <p role="status">Checking your academy session…</p>;
  if (session?.authenticated || session?.mfaRequired) return <p role="status">Opening your academy…</p>;
  if (mode === "email") return <div className="auth-credentials"><MagicLinkForm /><button type="button" className="text-link" onClick={() => changeMode("login")}>Sign in with a password instead</button></div>;
  if (sent) return <div className="auth-success" role="status"><span>Verify your email</span><h1>Your next move is in your inbox.</h1><p>Open the verification link to finish creating your account. After that, sign in with your username or email and password.</p><button className="text-link" onClick={() => changeMode("login")}>Return to sign in</button></div>;

  return <div className="auth-credentials">
    <form className="auth-form" onSubmit={submit}>
      <span className="eyebrow">Your academy account</span>
      <h1>{mode === "register" ? "Make your first move." : "Welcome back."}</h1>
      <p>{mode === "register" ? "Create a guardian account to begin your family’s chess journey." : "Sign in to keep your chess journey moving."}</p>
      {mode === "register" ? <>
        <label><span>Your name</span><input name="displayName" autoComplete="name" maxLength={100} required /></label>
        <label><span>Email address</span><input type="email" name="email" autoComplete="email" maxLength={254} required /></label>
        <label><span>Username</span><input name="username" autoComplete="username" pattern="[a-zA-Z0-9][a-zA-Z0-9_.\-]{2,31}" minLength={3} maxLength={32} required aria-describedby="username-hint" /></label><small id="username-hint">3–32 letters, numbers, dots, hyphens or underscores.</small>
      </> : <label><span>Username or email</span><input name="login" autoComplete="username" maxLength={254} required autoCapitalize="none" spellCheck={false} /></label>}
      <label><span>Password</span><input type="password" name="password" autoComplete={mode === "register" ? "new-password" : "current-password"} minLength={mode === "register" ? 12 : undefined} maxLength={128} required aria-describedby={mode === "register" ? "password-hint" : undefined} /></label>{mode === "register" && <small id="password-hint">Use 12–128 characters. A memorable passphrase works well.</small>}
      {(error || linkError) && <p className="form-message form-message-error" role="alert">{error || linkError}</p>}
      <button className="button button-gold button-wide" disabled={busy}>{busy ? "Please wait…" : mode === "register" ? "Create account" : "Sign in"}</button>
    </form>
    <GoogleSignIn onSuccess={complete} />
    <div className="auth-alternate"><span>{mode === "register" ? "Already have an account?" : "New to the academy?"}</span><button type="button" className="text-link" disabled={busy} onClick={() => changeMode(mode === "register" ? "login" : "register")}>{mode === "register" ? "Sign in" : "Create an account"}</button></div>
    <div className="auth-alternate"><span>Need help signing in?</span><button type="button" className="text-link" disabled={busy} onClick={() => changeMode("email")}>Email me a sign-in link</button></div>
    <div className="auth-alternate"><span>Student with an access code?</span><Link href="/student-access">Use student access</Link></div>
  </div>;
}
