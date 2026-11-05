import type { ReactNode } from "react";
import { getOrganization, getPolicySummaries } from "@/app/lib/public-data";
import PublicFooter from "./PublicFooter";
import PublicNavbar from "./PublicNavbar";
import "../../public.css";

export default async function PublicShell({ children }: { children: ReactNode }) {
  const [organization, policies] = await Promise.all([
    getOrganization(),
    getPolicySummaries(),
  ]);

  return (
    <div className="public-site">
      <PublicNavbar organization={organization} />
      {children}
      <PublicFooter organization={organization} policies={policies} />
    </div>
  );
}
