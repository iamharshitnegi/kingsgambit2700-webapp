"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import { requestMagicLink } from "@/app/lib/auth-client";
import { safeReturnPath } from "@/app/lib/role-routing";
import { isDemoMode } from "@/app/lib/demo/mode";

export default function MagicLinkForm() {
  const searchParams = useSearchParams();
  const [status, setStatus] = useState<"idle" | "sending" | "sent" | "error">("idle");
  const [message, setMessage] = useState("");

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const email = String(data.get("email") || "").trim();
    const next = safeReturnPath(searchParams.get("next"));
    const redirectPath = `/auth/complete?next=${encodeURIComponent(next)}`;
    setStatus("sending");
    setMessage("");
    try {
      await requestMagicLink(email, redirectPath);
      setStatus("sent");
    } catch (error) {
      setStatus("error");
      setMessage(error instanceof Error ? error.message : "We could not send the sign-in link.");
    }
  }

  if (status === "sent") {
    if (isDemoMode()) return <div className="auth-success" role="status"><span>Local account ready</span><h1>Your demo workspace is ready.</h1><p>No email is sent in local demo mode. Continue to explore the academy with sample data.</p><Link href={`/auth/complete?next=${encodeURIComponent(safeReturnPath(searchParams.get("next")))}`} className="button button-gold">Continue to workspace</Link></div>;
    return <div className="auth-success" role="status"><span>Check your inbox</span><h1>If the email belongs to an academy account, a secure link is on its way.</h1><p>For privacy, this message is the same whether or not an account exists. The link expires and can be used only as configured by the academy service.</p><button className="text-link" onClick={() => setStatus("idle")}>Use a different email</button></div>;
  }

  return (
    <form className="auth-form" onSubmit={handleSubmit}>
      <span className="eyebrow">Secure academy access</span>
      <h1>Sign in without another password.</h1>
      <p>We will send a one-time sign-in link to the email on your academy account.</p>
      <label><span>Email address</span><input type="email" name="email" autoComplete="email" required autoFocus /></label>
      {message && <p className="form-message form-message-error" role="alert">{message}</p>}
      <button className="button button-gold button-wide" disabled={status === "sending"}>{status === "sending" ? "Sending secure link…" : "Email me a sign-in link"}</button>
      <div className="auth-alternate"><span>Student with an access code?</span><Link href="/student-access">Use student access</Link></div>
      {isDemoMode() && <div className="auth-alternate"><span>Testing on your machine?</span><Link href="/demo">Choose a demo account →</Link></div>}
    </form>
  );
}
