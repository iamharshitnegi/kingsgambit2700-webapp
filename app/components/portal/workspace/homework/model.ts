import type { Assignment, LearningWorkspace } from "@/app/lib/academy-types";

export const activities = [
  { kind: "study", label: "Study & reflect", icon: "book", description: "Explore an idea and explain it in your own words." },
  { kind: "puzzle_set", label: "Chess puzzles", icon: "board", description: "Practise positions with moves checked as you go." },
  { kind: "game_review", label: "Game review", icon: "chart", description: "Find the turning point and reflect on a game." },
  { kind: "file_upload", label: "Practice evidence", icon: "homework", description: "Share a PGN, image, or PDF with your coach." },
  { kind: "other", label: "Written response", icon: "edit", description: "Answer a question or describe your thinking." },
];
export const activityFor = (kind: string) => activities.find(item => item.kind === kind) || activities[4];
export const statusLabels: Record<string, string> = {
  assigned: "Not started", started: "In progress", returned: "Needs another look",
  submitted: "With your coach", reviewed: "Completed", draft: "Draft", closed: "Closed",
};
export type HomeworkFilter = "todo" | "submitted" | "reviewed" | "draft" | "closed" | "all";
export function assignmentStatus(item: Assignment) {
  return ["closed", "draft"].includes(item.status) ? item.status : item.attempts.at(-1)?.status || item.status;
}
export const isTodo = (item: Assignment) => ["assigned", "started", "returned"].includes(assignmentStatus(item));
export const matchesFilter = (item: Assignment, filter: HomeworkFilter) => filter === "all" || (filter === "todo" ? isTodo(item) : assignmentStatus(item) === filter);
export const canPractice = (item: Assignment, student?: LearningWorkspace["students"][number]) => !!student?.canSubmit && isTodo(item);
export function dueLabel(due: string | null, timeZone?: string) {
  if (!due) return "No due date";
  return new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric", hour: "numeric", minute: "2-digit", timeZone }).format(new Date(due));
}
export function localDateInput(due?: string | null) {
  if (!due) return "";
  const date = new Date(due);
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
}

// A returned attempt stays in history; the next practice starts with fresh puzzles.
export function currentAttempt(item: Assignment) {
  const attempt = item.attempts.at(-1);
  return attempt?.status === "returned" ? undefined : attempt;
}
