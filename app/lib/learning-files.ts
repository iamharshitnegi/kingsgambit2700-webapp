import type { LearningFile, LearningWorkspace } from "./academy-types";
import { APIError, authenticatedFetch, getAPIBaseURL, getCSRFToken } from "./api/client";
import { isDemoMode } from "./demo/mode";

async function fileRequest(path: string, init: RequestInit): Promise<Response> {
  const base = getAPIBaseURL();
  if (!base) throw new APIError("The academy service is not configured.", 503);
  const headers = new Headers(init.headers);
  const token = getCSRFToken();
  if (init.method === "POST" && token) headers.set("X-CSRF-Token", token);
  const request = { ...init, headers, credentials: "include" as const, cache: "no-store" as const };
  const response = isDemoMode()
    ? await (await import("./demo/transport")).demoFetch(`${base}${path}`, request)
    : await authenticatedFetch(new Request(`${base}${path}`, request));
  if (!response.ok) {
    const problem = await response.json().catch(() => null);
    throw new APIError(problem?.detail || problem?.title || `File request failed (${response.status}).`, response.status, problem);
  }
  return response;
}

export async function uploadLearningFile(form: FormData, key: string): Promise<{ file: LearningFile; workspace: LearningWorkspace }> {
  const response = await fileRequest("/learning/files", { method: "POST", body: form, headers: { "Idempotency-Key": key } });
  return response.json();
}

export async function downloadLearningFile(id: string, filename: string) {
  const response = await fileRequest(`/learning/files/${encodeURIComponent(id)}/download`, { method: "GET" });
  const url = URL.createObjectURL(await response.blob());
  const link = document.createElement("a");
  link.href = url; link.download = filename; link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
