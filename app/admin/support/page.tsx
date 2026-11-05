"use client";

import PortalGate from "@/app/components/portal/PortalGate";
import PortalShell from "@/app/components/portal/PortalShell";
import SupportDesk from "@/app/components/portal/SupportDesk";
import type { AcademyRole } from "@/app/lib/api/types";

const ALLOWED: AcademyRole[] = ["operations_admin", "super_admin"];

export default function AdminSupportPage() {
  return (
    <PortalGate allowedRoles={ALLOWED}>
      {(user) => (
        <PortalShell user={user} eyebrow="Operations" title="Family support">
          <SupportDesk canManage />
        </PortalShell>
      )}
    </PortalGate>
  );
}
