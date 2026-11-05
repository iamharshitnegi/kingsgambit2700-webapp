"use client";

import { apiFetch, setCSRFToken, setSessionRecovery } from "./api/client";
import { publishSession, publishSessionError, readSessionState } from "./session-state";
import {
  createTOTPEnrollment,
  endSession,
  fetchMFAStatus,
  fetchSession,
  redeemStudentCode,
  sendMagicLink,
  verifyMFAChallenge,
  verifyTOTPEnrollment,
} from "./api/typed-client";
import type { MFAChallenge, SessionResponse } from "./api/types";
import { isDemoMode } from "./demo/mode";

let pendingSession: Promise<SessionResponse> | undefined;
let sessionGeneration = 0;

function accept(session: SessionResponse) {
  setCSRFToken(session.csrfToken);
  publishSession(session);
  return session;
}

function changed(session: SessionResponse) {
  sessionGeneration += 1;
  pendingSession = undefined;
  return accept(session);
}

export async function signInWithPassword(login: string, password: string) {
  const session = await apiFetch<SessionResponse>("/auth/password/login", {
    method: "POST", body: JSON.stringify({ login, password }),
  });
  return changed(session);
}

export function registerWithPassword(input: { email: string; username: string; displayName: string; password: string; redirectPath: string }) {
  return apiFetch<{ status: string }>("/auth/password/register", { method: "POST", body: JSON.stringify(input) });
}

export function getGoogleConfiguration() {
  return apiFetch<{ enabled: boolean; clientId?: string; nonce?: string }>("/auth/google/config");
}

export async function signInWithGoogle(credential: string) {
  const session = await apiFetch<SessionResponse>("/auth/google", { method: "POST", body: JSON.stringify({ credential }) });
  return changed(session);
}

export function getSession(): Promise<SessionResponse> {
  if (pendingSession) return pendingSession;
  const generation = sessionGeneration;
  const work = (async () => {
    try {
      let session = await fetchSession();
      if (!session.authenticated && session.restoreAvailable && generation === sessionGeneration) {
        setCSRFToken(session.csrfToken);
        session = await apiFetch<SessionResponse>("/auth/session/restore", { method: "POST" });
      }
      if (generation !== sessionGeneration) return readSessionState().session || getSession();
      accept(session);
      return session;
    } catch (error) {
      if (generation !== sessionGeneration) return readSessionState().session || getSession();
      if (generation === sessionGeneration) publishSessionError(error);
      throw error;
    } finally {
      if (generation === sessionGeneration) pendingSession = undefined;
    }
  })();
  pendingSession = work;
  return work;
}

setSessionRecovery(async () => (await getSession()).authenticated);

export async function getMFAStatus() {
  const generation = sessionGeneration;
  const status = await fetchMFAStatus();
  if (generation === sessionGeneration) setCSRFToken(status.csrfToken);
  return status;
}

export function beginTOTPEnrollment() {
  return createTOTPEnrollment();
}

export async function confirmTOTPEnrollment(code: string) {
  const session = await verifyTOTPEnrollment(code);
  changed(session);
  return session;
}

export async function completeMFAChallenge(challenge: MFAChallenge) {
  const session = await verifyMFAChallenge(challenge);
  return changed(session);
}

export function requestMagicLink(email: string, redirectPath = "/auth/complete") {
  return sendMagicLink(email, redirectPath);
}

export async function exchangeStudentCode(code: string) {
  const session = await redeemStudentCode(code);
  return changed(session);
}

export function clearSession() {
  return changed({ authenticated: false });
}

export async function logout() {
  await getSession();
  await endSession();
  clearSession();
  if (!isDemoMode() && typeof BroadcastChannel !== "undefined") {
    const channel = new BroadcastChannel("academy-session");
    channel.postMessage("signed-out");
    channel.close();
  }
}
