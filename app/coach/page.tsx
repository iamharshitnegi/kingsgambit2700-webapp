"use client";
import PortalGate from "../components/portal/PortalGate";
import PortalShell from "../components/portal/PortalShell";
import RoleDashboard from "../components/portal/RoleDashboard";
import type { AcademyRole } from "../lib/api/types";
const ALLOWED: AcademyRole[] = ["coach"];
export default function CoachPage() { return <PortalGate allowedRoles={ALLOWED}>{(user) => <PortalShell user={user} eyebrow="Coach workspace" title="Today"><RoleDashboard kind="coach" user={user} /></PortalShell>}</PortalGate>; }
