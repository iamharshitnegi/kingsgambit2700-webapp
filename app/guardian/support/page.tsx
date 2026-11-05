"use client";

import PortalGate from "@/app/components/portal/PortalGate";
import PortalShell from "@/app/components/portal/PortalShell";
import SupportDesk from "@/app/components/portal/SupportDesk";
import type { AcademyRole } from "@/app/lib/api/types";

const ALLOWED: AcademyRole[] = ["guardian"];

export default function GuardianSupportPage() {
  return (
    <PortalGate allowedRoles={ALLOWED}>
      {(user) => (
        <PortalShell user={user} eyebrow="Family help" title="Support">
          <SupportDesk canManage={false} />
        </PortalShell>
      )}
    </PortalGate>
  );
}
