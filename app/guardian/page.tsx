"use client";
import PortalGate from "../components/portal/PortalGate";
import PortalShell from "../components/portal/PortalShell";
import RoleDashboard from "../components/portal/RoleDashboard";
import type { AcademyRole } from "../lib/api/types";
const ALLOWED: AcademyRole[] = ["guardian"];
export default function GuardianPage() { return <PortalGate allowedRoles={ALLOWED}>{(user) => <PortalShell user={user} eyebrow="Guardian home" title="Family overview"><RoleDashboard kind="guardian" user={user} /></PortalShell>}</PortalGate>; }
