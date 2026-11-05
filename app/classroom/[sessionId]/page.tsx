"use client";
import { use } from "react";
import DemoClassroom from "../../components/classroom/DemoClassroom";
import { isDemoMode } from "../../lib/demo/mode";
import ClassroomClient from "../../components/classroom/ClassroomClient";
import PortalGate from "../../components/portal/PortalGate";
import PortalShell from "../../components/portal/PortalShell";
import type { AcademyRole } from "../../lib/api/types";

const ALLOWED: AcademyRole[] = ["student", "coach", "academic_manager", "operations_admin", "super_admin"];

export default function ClassroomPage({ params }: { params: Promise<{ sessionId: string }> }) {
  const { sessionId } = use(params);
  return (
    <PortalGate allowedRoles={ALLOWED}>
      {(user) => (
        <PortalShell user={user} eyebrow="Protected classroom" title="Live classroom">
          {isDemoMode() ? <DemoClassroom sessionId={sessionId} user={user} /> : <ClassroomClient sessionId={sessionId} />}
        </PortalShell>
      )}
    </PortalGate>
  );
}
