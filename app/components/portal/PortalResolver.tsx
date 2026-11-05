"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { getSession } from "@/app/lib/auth-client";
import type { SessionUser } from "@/app/lib/api/types";
import { landingPathForRole, roleLabels } from "@/app/lib/role-routing";

export default function PortalResolver() {
  const router = useRouter();
  const [message, setMessage] = useState("Checking your academy access…");
  const [user, setUser] = useState<SessionUser | null>(null);

  useEffect(() => {
    let active = true;
    getSession()
      .then((session) => {
        if (!active) return;
        if (!session.authenticated || !session.user) {
          router.replace(session.mfaRequired ? "/auth/complete?mfa=required&next=%2Fportal" : "/sign-in?next=%2Fportal");
          return;
        }
        if (session.user.roles.length === 0) {
          router.replace("/forbidden");
          return;
        }
        if (session.user.roles.length === 1) {
          router.replace(landingPathForRole(session.user.roles[0]));
          return;
        }
        setUser(session.user);
      })
      .catch(() => active && setMessage("We could not verify your session. Please refresh or return to sign in."));
    return () => { active = false; };
  }, [router]);

  if (user) {
    return (
      <main className="portal-center">
        <section className="portal-status-card portal-role-selector">
          <span className="eyebrow">Choose a workspace</span>
          <h1>How are you entering the academy today?</h1>
          <p>Choose the workspace you need. You can switch views at any time.</p>
          <div className="role-choice-grid">
            {user.roles.map((role) => (
              <Link key={role} href={landingPathForRole(role)}>
                <strong>{roleLabels[role]}</strong>
                <span>Open workspace →</span>
              </Link>
            ))}
          </div>
        </section>
      </main>
    );
  }

  return <main className="portal-center" aria-live="polite"><div className="portal-loader" aria-hidden="true" /><p>{message}</p></main>;
}
