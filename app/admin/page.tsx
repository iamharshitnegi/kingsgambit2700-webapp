"use client";
import PortalGate from "../components/portal/PortalGate";
import PortalShell from "../components/portal/PortalShell";
import RoleDashboard from "../components/portal/RoleDashboard";
import { ADMIN_ROLES } from "../lib/role-routing";
export default function AdminPage() { return <PortalGate allowedRoles={ADMIN_ROLES}>{(user) => <PortalShell user={user} eyebrow="Academy operations" title="Command centre"><RoleDashboard kind="admin" user={user} /></PortalShell>}</PortalGate>; }
