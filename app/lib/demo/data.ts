import type { DemoLearningState } from "./learning";
import type { DemoAcademyState } from "./academy";
import type { AcademyRole, SessionUser } from "../api/types";
import type { SupportCase } from "../api/typed-client";

export const DEMO_FAMILY_ID = "01990000-0000-7000-8000-000000000001";
export const DEMO_STUDENT_ID = "01990000-0000-7000-8000-000000000002";
export const DEMO_CLASS_ID = "01990000-0000-7000-8000-000000000003";
export const DEMO_STORAGE_KEY = "kingsgambit.demo.v1";
export const DEMO_SESSION_KEY = "kingsgambit.demo.session.v1";

export type DemoStudent = { id: string; name: string; ageBand: string; level: string; consent: boolean; consentVersion?: string; consentAt?: string };
export type DemoClass = { id: string; title: string; coach: string; studentIds: string[]; startsAt: string; endsAt: string; status: "scheduled" | "completed" | "cancelled"; attendance: Record<string, "present" | "absent" | "excused">; notes: string; format: string };
export type DemoAssignment = { id: string; studentId: string; title: string; description: string; dueAt: string; status: "assigned" | "submitted" | "reviewed"; answer?: string; feedback?: string };
export type DemoInvoice = { id: string; description: string; amountMinor: number; currency: string; status: "due" | "paid"; dueAt: string; paidAt?: string; credits: number };
export type DemoReport = { id: string; studentId: string; title: string; summary: string; skills: { name: string; score: number }[]; status: "draft" | "published"; acknowledgedAt?: string };
export type DemoLead = { id: string; guardianName: string; email?: string; phone?: string; status: string; createdAt: string; studentAgeBand?: string };
export type DemoBooking = { id: string; leadId: string; slotId: string; guardianName: string; email: string; startsAt: string; endsAt: string; timeZone: string; status: string; idempotencyKey: string };
export type DemoWorkspace = {
  learning?: DemoLearningState;
  academy?: DemoAcademyState;
  family: { id: string; name: string; timeZone: string; credits: number };
  students: DemoStudent[];
  classes: DemoClass[];
  assignments: DemoAssignment[];
  invoices: DemoInvoice[];
  reports: DemoReport[];
  leads: DemoLead[];
  bookings: DemoBooking[];
  supportCases: SupportCase[];
  audit: { id: string; actor: string; action: string; at: string }[];
  boards?: Record<string, DemoWorkspace["board"]>;
  board: { fen: string; pgn: string; revision: number; studentControl: boolean };
};

export const demoAccounts: { role: AcademyRole; name: string; email: string; description: string }[] = [
  { role: "guardian", name: "Priya Sharma", email: "guardian@example.test", description: "Children, consent, classes, progress and billing" },
  { role: "student", name: "Aarav", email: "student@example.test", description: "Join a lesson, practise and submit homework" },
  { role: "coach", name: "Coach Maya", email: "coach@example.test", description: "Teach, mark attendance and give feedback" },
  { role: "operations_admin", name: "Alex · Operations", email: "operations@example.test", description: "Trial bookings, leads and family support" },
  { role: "academic_manager", name: "Sam · Academics", email: "academics@example.test", description: "Review learning and publish progress reports" },
  { role: "finance_admin", name: "Jordan · Finance", email: "finance@example.test", description: "Invoices, payments and class credits" },
  { role: "super_admin", name: "Admin", email: "admin@example.test", description: "Explore all academy operations and audit history" },
];

export function demoUser(role: AcademyRole): SessionUser {
  const account = demoAccounts.find((item) => item.role === role);
  if (!account) throw new Error("Choose an available demo account.");
  return { id: role === "student" ? DEMO_STUDENT_ID : `demo-${role}`, displayName: account.name, ...(role === "student" ? {} : { email: account.email }), roles: [role] };
}

export function createDemoWorkspace(now = Date.now()): DemoWorkspace {
  const iso = (hours: number) => new Date(now + hours * 3_600_000).toISOString();
  return {
    family: { id: DEMO_FAMILY_ID, name: "Sharma family", timeZone: "Asia/Kolkata", credits: 8 },
    students: [{ id: DEMO_STUDENT_ID, name: "Aarav", ageBand: "8_12", level: "Beginner · Developing foundations", consent: true, consentVersion: "demo-1", consentAt: iso(-168) }],
    classes: [
      { id: DEMO_CLASS_ID, title: "Finding the winning move", coach: "Coach Maya", studentIds: [DEMO_STUDENT_ID], startsAt: iso(-0.05), endsAt: iso(0.95), status: "scheduled", attendance: {}, notes: "", format: "Small group" },
      { id: "demo-class-next", title: "Rooks on open files", coach: "Coach Maya", studentIds: [DEMO_STUDENT_ID], startsAt: iso(48), endsAt: iso(49), status: "scheduled", attendance: {}, notes: "", format: "Small group" },
      { id: "demo-class-past", title: "The power of a fork", coach: "Coach Maya", studentIds: [DEMO_STUDENT_ID], startsAt: iso(-72), endsAt: iso(-71), status: "completed", attendance: { [DEMO_STUDENT_ID]: "present" }, notes: "Aarav spotted knight forks confidently. Next: check opponent threats before moving.", format: "Small group" },
    ],
    assignments: [{ id: "demo-assignment-1", studentId: DEMO_STUDENT_ID, title: "Look for checks, captures and threats", description: "After 1. e4 e5 2. Nf3 Nc6 3. Bc4, explain which square the bishop targets and why it is vulnerable. Write your thinking below.", dueAt: iso(48), status: "assigned" }],
    invoices: [{ id: "KG-DEMO-1001", description: "Small-group academy · 8 classes", amountMinor: 320000, currency: "INR", status: "due", dueAt: iso(72), credits: 8 }],
    reports: [
      { id: "demo-report-1", studentId: DEMO_STUDENT_ID, title: "Your first month of progress", summary: "Aarav has built a strong foundation in piece safety and tactical patterns. We will focus next on slowing down before captures and explaining a plan.", skills: [{ name: "Piece safety", score: 80 }, { name: "Tactical patterns", score: 65 }, { name: "Planning", score: 45 }], status: "published" },
      { id: "demo-report-draft", studentId: DEMO_STUDENT_ID, title: "Building a thinking routine", summary: "Aarav is beginning to check forcing moves before choosing a plan. Next month we will practise describing the opponent’s best reply.", skills: [{ name: "Piece safety", score: 85 }, { name: "Tactical patterns", score: 72 }, { name: "Planning", score: 55 }], status: "draft" },
    ],
    leads: [{ id: "demo-lead-1", guardianName: "Riya Patel", email: "riya@example.test", status: "new", createdAt: iso(-2), studentAgeBand: "8_12" }],
    bookings: [], supportCases: [],
    audit: [{ id: "demo-audit-1", actor: "Demo setup", action: "Sample academy created", at: iso(-168) }],
    boards: {},
    board: { fen: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", pgn: "", revision: 0, studentControl: false },
  };
}
