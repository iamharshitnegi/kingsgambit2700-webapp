import type { AcademyRole } from "./api/types";

import type { components } from "./api/schema";

export type AcademyStaff = components["schemas"]["AcademyStaffRecord"];
export type AcademyStaffDirectory = components["schemas"]["AcademyStaffDirectory"];
export type AcademyStudent = components["schemas"]["AcademyStudent"];
export type AcademyBatch = components["schemas"]["AcademyBatch"];
export type AcademyEnrolment = components["schemas"]["AcademyEnrolment"];
export type AcademyClass = components["schemas"]["AcademyClass"];
export type AcademyWorkspaceData = components["schemas"]["AcademyWorkspace"];
export type AcademySavedView = components["schemas"]["AcademySavedView"];
export type AcademyAvailability = components["schemas"]["AcademyAvailability"];
export type AcademyLead = components["schemas"]["AcademyLead"];
export type AcademyTrialSlot = components["schemas"]["AcademyTrialSlot"];
export type AcademyTrialBooking = components["schemas"]["AcademyTrialBooking"];
export type AcademyOperationsData = components["schemas"]["AcademyOperations"];
export type ClassInput = components["schemas"]["AcademyClassInput"];
export type CancellationPreview =
  components["schemas"]["AcademyCancellationPreview"];
export type Puzzle = components["schemas"]["LearningPuzzle"];
export type PuzzleProgress = components["schemas"]["LearningPuzzleProgress"];
export type LearningMetrics = components["schemas"]["LearningMetrics"];
export type LearningFile = components["schemas"]["LearningFile"];
export type LearningAttempt = components["schemas"]["LearningAttempt"];
export type AssignedResource =
  components["schemas"]["LearningAssignedResource"];
export type Assignment = components["schemas"]["LearningAssignment"];
export type LearningResource = components["schemas"]["LearningResource"];
export type LearningGame = components["schemas"]["LearningGame"];
export type LearningThread = components["schemas"]["LearningThread"];
export type Rubric = components["schemas"]["LearningRubric"];
export type AssessmentContent =
  components["schemas"]["LearningAssessmentContent"];
export type AssessmentEvent = components["schemas"]["LearningAssessmentEvent"];
export type Assessment = components["schemas"]["LearningAssessment"];
export type LearningPlacement = components["schemas"]["LearningPlacement"];
export type LearningWorkspace = components["schemas"]["LearningWorkspace"];
export type LearningAction = (
  action: string,
  input: Record<string, unknown>,
  message?: string,
) => Promise<boolean>;
export type AcademyMutation = (
  path: string,
  input: Record<string, unknown>,
  message: string,
  method?: "POST" | "PATCH" | "PUT",
) => Promise<boolean>;
export const workspaceSections = [
  "home",
  "calendar",
  "classes",
  "students",
  "homework",
  "board",
  "library",
  "games",
  "progress",
  "messages",
  "notifications",
  "settings",
  "operations",
] as const;
export type WorkspaceSection = (typeof workspaceSections)[number];
export const sectionLabels: Record<WorkspaceSection, string> = {
  home: "Overview",
  calendar: "Calendar",
  classes: "Classes",
  students: "Learners & batches",
  homework: "Homework",
  board: "Teaching board",
  library: "Resource library",
  games: "Game reviews",
  progress: "Progress & feedback",
  messages: "Messages",
  notifications: "Notifications",
  settings: "Settings & access",
  operations: "Trials & operations",
};
export const academyLevels = [
  { slug: "foundation-1", name: "Board foundations" },
  { slug: "foundation-2", name: "Tactical foundations" },
  { slug: "development-1", name: "Calculation and planning" },
  { slug: "development-2", name: "Competitive habits" },
  { slug: "performance-1", name: "Advanced performance" },
];
export const staffRoles: AcademyRole[] = [
  "coach",
  "academic_manager",
  "operations_admin",
  "finance_admin",
  "super_admin",
];
