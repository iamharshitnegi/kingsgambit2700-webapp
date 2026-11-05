"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { exchangeStudentCode } from "@/app/lib/auth-client";
import { isDemoMode } from "@/app/lib/demo/mode";
import { landingPathForRoles } from "@/app/lib/role-routing";

export default function StudentCodeForm() {
  const router = useRouter();
  const [status, setStatus] = useState<"idle" | "checking" | "error">("idle");
  const [message, setMessage] = useState("");

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const code = String(data.get("code") || "").trim().replace(/\s+/g, "");
    setStatus("checking");
    setMessage("");
    try {
      const session = await exchangeStudentCode(code);
      if (!session.authenticated || !session.user) throw new Error("This access code could not open a student session.");
      router.replace(landingPathForRoles(session.user.roles));
    } catch (error) {
      setStatus("error");
      setMessage(error instanceof Error ? error.message : "This code could not be verified.");
    }
  }

  return <form className="auth-form" onSubmit={handleSubmit}><span className="eyebrow">Student access</span><h1>Enter the code from your guardian or academy.</h1><p>A student code opens only the assigned student account. Never share it in a class chat.</p>{isDemoMode() && <p className="mfa-security-note">Local demo access code: <strong>DEMO2700</strong></p>}<label><span>Student access code</span><input name="code" autoComplete="one-time-code" inputMode="text" minLength={6} required autoFocus className="code-input" /></label>{message && <p className="form-message form-message-error" role="alert">{message}</p>}<button className="button button-gold button-wide" disabled={status === "checking"}>{status === "checking" ? "Checking code…" : "Open student portal"}</button><div className="auth-alternate"><span>Guardian, coach, or staff member?</span><Link href="/sign-in">Sign in by email</Link></div></form>;
}
