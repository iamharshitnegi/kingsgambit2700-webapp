"use client";
import { use, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useSession } from "@/app/lib/use-session";
import { workspaceHome } from "@/app/lib/role-routing";
import { workspaceSections, type WorkspaceSection } from "@/app/lib/academy-types";
export default function Page({ params }: { params: Promise<{ section: string }> }) {
  const { section: target } = use(params); const router = useRouter(); const { session, error } = useSession();
  const section = target === "learning" ? "homework" : target;
  useEffect(() => {
    if (!session) return;
    if (session.authenticated && session.user) router.replace(`${workspaceHome(session.user)}/${workspaceSections.includes(section as WorkspaceSection) ? section : "home"}`);
    else router.replace(`${session.mfaRequired ? "/auth/complete?mfa=required&" : "/sign-in?"}next=${encodeURIComponent(`/portal/${section}`)}`);
  }, [session, section, router]);
  return <main className="portal-center"><p role={error ? "alert" : "status"}>{error || "Opening your workspace…"}</p>{error && <button onClick={() => window.location.reload()}>Try again</button>}</main>;
}
