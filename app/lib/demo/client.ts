import { apiFetch } from "../api/client";
import type { DemoWorkspace } from "./data";

export function fetchWorkspace(classId?: string) {
  return apiFetch<DemoWorkspace>(`/demo/workspace${classId ? `?classId=${encodeURIComponent(classId)}` : ""}`);
}

export function demoAction(action: string, input: Record<string, unknown> = {}) {
  return apiFetch<DemoWorkspace>("/demo/actions", { method: "POST", body: JSON.stringify({ action, ...input }) });
}
