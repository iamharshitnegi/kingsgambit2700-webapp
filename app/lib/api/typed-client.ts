import createClient from "openapi-fetch";
import type { paths, components } from "./schema";
import { APIError, apiFetch, authenticatedFetch, getAPIBaseURL, getCSRFToken } from "./client";
import { isDemoMode } from "../demo/mode";
import type {
  AgeBand,
  LeadInput,
  LeadResponse,
  MFAChallenge,
  MFAEnrollmentCompletion,
  MFAStatus,
  SessionResponse,
  TOTPEnrollment,
  TrialBookingInput,
  TrialBookingResponse,
  TrialFormat,
  TrialSlot,
} from "./types";

type Problem = components["schemas"]["Problem"];

export type SupportCase = components["schemas"]["SupportCase"];
export type SupportCasePage = components["schemas"]["SupportCasePage"];
export type CreateSupportCase = components["schemas"]["CreateSupportCase"];
export type AppendSupportEvent = components["schemas"]["AppendSupportEvent"];
export type AssignSupportCase = components["schemas"]["AssignSupportCase"];
export type ChangeSupportCaseStatus = components["schemas"]["ChangeSupportCaseStatus"];

function academyClient({ publicCache = false, revalidate = 300 }: { publicCache?: boolean; revalidate?: number } = {}) {
  const baseUrl = getAPIBaseURL();
  if (!baseUrl) throw new APIError("The academy service is not configured for this environment.", 503);

  const client = createClient<paths>({
    baseUrl,
    credentials: "include",
    fetch: async (request) => {
      if (isDemoMode()) return (await import("../demo/transport")).demoFetch(request);
      try {
        return publicCache ? await fetch(request, { next: { revalidate } } as RequestInit) : await authenticatedFetch(request);
      } catch {
        throw new APIError("The academy service could not be reached. Please try again.", 503);
      }
    },
  });

  client.use({
    onRequest({ request }) {
      request.headers.set("Accept", "application/json");
      if (!["GET", "HEAD", "OPTIONS"].includes(request.method)) {
        const token = getCSRFToken();
        if (token) request.headers.set("X-CSRF-Token", token);
      }
      return request;
    },
  });

  return client;
}

export function unwrapAPIResponse<T>({ data, error, response }: { data?: T; error?: unknown; response: Response }): T {
  if (error !== undefined) {
    const problem = error && typeof error === "object" ? (error as Problem) : undefined;
    throw new APIError(problem?.detail || problem?.title || `Request failed (${response.status}).`, response.status, problem);
  }
  return data as T;
}

export function getPublicAPIClient() {
  return academyClient({ publicCache: true });
}

export async function fetchPolicyVersion(slug: string, version: string) {
  const result = await academyClient({ publicCache: true, revalidate: 31_536_000 })
    .GET("/public/policies/{slug}/versions/{version}", { params: { path: { slug, version } } });
  return unwrapAPIResponse(result);
}

export async function fetchSession(): Promise<SessionResponse> {
  const result = await academyClient().GET("/auth/session");
  return unwrapAPIResponse(result);
}

export function fetchMFAStatus(): Promise<MFAStatus> {
  return apiFetch<MFAStatus>("/auth/mfa/status", { cache: "no-store" });
}

export function createTOTPEnrollment(): Promise<TOTPEnrollment> {
  return apiFetch<TOTPEnrollment>("/auth/mfa/totp/enrollment", {
    method: "POST",
  });
}

export function verifyTOTPEnrollment(code: string): Promise<MFAEnrollmentCompletion> {
  return apiFetch<MFAEnrollmentCompletion>("/auth/mfa/totp/enrollment/confirm", {
    method: "POST",
    body: JSON.stringify({ code }),
  });
}

export function verifyMFAChallenge(challenge: MFAChallenge): Promise<SessionResponse> {
  return apiFetch<SessionResponse>("/auth/mfa/challenge", {
    method: "POST",
    body: JSON.stringify(challenge),
  });
}

export async function fetchCurrentIdentity() {
  const result = await academyClient().GET("/me");
  return unwrapAPIResponse(result);
}

export async function createRealtimeJoinTicket(classSessionId: string) {
  const result = await academyClient().POST("/realtime/join-tickets", {
    body: { classSessionId },
  });
  return unwrapAPIResponse(result);
}

export async function sendMagicLink(email: string, redirectPath?: string) {
  const result = await academyClient().POST("/auth/magic-links", { body: { email, redirectPath } });
  return unwrapAPIResponse(result);
}

export async function redeemStudentCode(code: string): Promise<SessionResponse> {
  const result = await academyClient().POST("/auth/student-codes/exchange", { body: { code } });
  return unwrapAPIResponse(result);
}

export async function endSession() {
  const result = await academyClient().POST("/auth/logout");
  if (result.error !== undefined) unwrapAPIResponse(result);
}

export async function createLead(input: LeadInput): Promise<LeadResponse> {
  const result = await academyClient().POST("/public/leads", {
    body: input as components["schemas"]["CreateLead"],
  });
  return unwrapAPIResponse(result);
}

export async function listTrialSlots(query: {
  timezone: string;
  format: TrialFormat;
  from: string;
  to: string;
  ageBand?: AgeBand;
}): Promise<TrialSlot[]> {
  const result = await academyClient().GET("/public/trial-slots", { params: { query } });
  return unwrapAPIResponse(result).items;
}

export async function createTrialBooking(input: TrialBookingInput, idempotencyKey: string): Promise<TrialBookingResponse> {
  const result = await academyClient().POST("/public/trial-bookings", {
    params: { header: { "Idempotency-Key": idempotencyKey } },
    body: input,
  });
  return unwrapAPIResponse(result);
}

export async function listSupportCases(familyId: string, cursor?: string): Promise<SupportCasePage> {
  const result = await academyClient().GET("/support/cases", {
    params: { query: { familyId, cursor, limit: 20 } },
  });
  return unwrapAPIResponse(result);
}

export async function getSupportCase(caseId: string): Promise<SupportCase> {
  const result = await academyClient().GET("/support/cases/{caseId}", {
    params: { path: { caseId } },
  });
  return unwrapAPIResponse(result);
}

export async function createSupportCase(input: CreateSupportCase): Promise<SupportCase> {
  const result = await academyClient().POST("/support/cases", { body: input });
  return unwrapAPIResponse(result);
}

export async function appendSupportCaseEvent(
  caseId: string,
  input: AppendSupportEvent,
): Promise<SupportCase> {
  const result = await academyClient().POST("/support/cases/{caseId}/events", {
    params: { path: { caseId } },
    body: input,
  });
  return unwrapAPIResponse(result);
}

export async function assignSupportCase(
  caseId: string,
  input: AssignSupportCase,
): Promise<SupportCase> {
  const result = await academyClient().PATCH("/support/cases/{caseId}/assignment", {
    params: { path: { caseId } },
    body: input,
  });
  return unwrapAPIResponse(result);
}

export async function changeSupportCaseStatus(
  caseId: string,
  input: ChangeSupportCaseStatus,
): Promise<SupportCase> {
  const result = await academyClient().PATCH("/support/cases/{caseId}/status", {
    params: { path: { caseId } },
    body: input,
  });
  return unwrapAPIResponse(result);
}
