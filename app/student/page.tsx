"use client";
import PortalGate from "../components/portal/PortalGate";
import PortalShell from "../components/portal/PortalShell";
import RoleDashboard from "../components/portal/RoleDashboard";
import type { AcademyRole } from "../lib/api/types";
const ALLOWED: AcademyRole[] = ["student"];
export default function StudentPage() { return <PortalGate allowedRoles={ALLOWED}>{(user) => <PortalShell user={user} eyebrow="Student home" title="Your next move"><RoleDashboard kind="student" user={user} /></PortalShell>}</PortalGate>; }
