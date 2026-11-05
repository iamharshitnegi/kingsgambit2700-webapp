"use client";
import PortalGate from "../PortalGate";
import PortalShell from "../PortalShell";
import AcademyWorkspace from "./AcademyWorkspace";
import { sectionLabels, type WorkspaceSection } from "@/app/lib/academy-types";
import { ADMIN_ROLES } from "@/app/lib/role-routing";
import type { AcademyRole } from "@/app/lib/api/types";
const roles: Record<string, AcademyRole[]> = {
  admin: ADMIN_ROLES,
  coach: ["coach"],
  guardian: ["guardian"],
  student: ["student"],
};
const descriptions: Partial<Record<WorkspaceSection, string>> = {
  students:
    "Every learner, their people, and their next step. Manage rosters, enrollment, and class credits.",
  homework:
    "Keep practice purposeful. Find assignments, follow attempts, and share useful feedback.",
  board:
    "A space to prepare, explore, and review. Your private board stays separate from the live classroom.",
  library:
    "Good lessons start here. Find positions, games, and study material for the next session.",
  games:
    "Return to the moves that matter. Replay saved games and keep coach feedback close.",
  progress:
    "See the learning behind the results. Review feedback, reports, and curriculum milestones.",
  messages:
    "Keep your learning conversations together, with the right people in the loop.",
  notifications:
    "Schedule updates, feedback, and the things that need your attention.",
  operations:
    "From first enquiry to the first lesson. Keep trials and follow-ups moving.",
  settings:
    "Make the workspace yours. Manage preferences, account access, and academy settings.",
};
export default function WorkspacePage({
  kind,
  section,
}: {
  kind: "admin" | "coach" | "guardian" | "student";
  section: WorkspaceSection;
}) {
  return (
    <PortalGate allowedRoles={roles[kind]}>
      {(user) => (
        <PortalShell
          user={user}
          eyebrow="Your academy workspace"
          title={section === "homework" && kind === "student" ? "My homework" : sectionLabels[section]}
          description={descriptions[section]}
        >
          <AcademyWorkspace user={user} kind={kind} section={section} />
        </PortalShell>
      )}
    </PortalGate>
  );
}
