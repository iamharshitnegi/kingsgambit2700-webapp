"use client";

import type { SessionUser } from "@/app/lib/api/types";
import { isDemoMode } from "@/app/lib/demo/mode";
import AcademyWorkspace from "./workspace/AcademyWorkspace";
import DemoBilling from "./workspace/DemoBilling";

export default function RoleDashboard({ kind, user }: { kind: "guardian" | "student" | "coach" | "admin"; user: SessionUser }) {
  const billing = isDemoMode() && user.roles.some((role) => ["guardian", "finance_admin", "super_admin"].includes(role));
  return <><AcademyWorkspace user={user} kind={kind} />{billing && <DemoBilling />}</>;
}
