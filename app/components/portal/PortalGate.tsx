"use client";

import { usePathname, useRouter } from "next/navigation";
import { useEffect, type ReactNode } from "react";
import { useSession } from "@/app/lib/use-session";
import type { AcademyRole, SessionUser } from "@/app/lib/api/types";
import { userCanAccess } from "@/app/lib/role-routing";

export default function PortalGate({
  allowedRoles,
  children,
}: {
  allowedRoles: AcademyRole[];
  children: (user: SessionUser) => ReactNode;
}) {
  const router = useRouter();
  const pathname = usePathname();
  const { session, error } = useSession();

  useEffect(() => {
    if (session) {
        if (!session.authenticated || !session.user) {
          router.replace(session.mfaRequired
            ? `/auth/complete?mfa=required&next=${encodeURIComponent(pathname)}`
            : `/sign-in?next=${encodeURIComponent(pathname)}`);
          return;
        }
        if (!userCanAccess(session.user, allowedRoles)) {
          router.replace("/forbidden");
          return;
        }
    }
  }, [allowedRoles, pathname, router, session]);

  if (session?.authenticated && session.user && userCanAccess(session.user, allowedRoles)) return children(session.user);

  if (error) {
    return (
      <main className="portal-center">
        <div className="portal-status-card" role="alert">
          <span className="eyebrow">Connection interrupted</span>
          <h1>We could not open your portal</h1>
          <p>{error} Check your connection and try again.</p>
          <button className="button button-dark" onClick={() => window.location.reload()}>
            Try again
          </button>
        </div>
      </main>
    );
  }

  return (
    <main className="portal-center" aria-busy="true" aria-live="polite">
      <div className="portal-loader" aria-hidden="true" />
      <p>{session ? "Taking you to the right place…" : "Opening your academy portal…"}</p>
    </main>
  );
}
