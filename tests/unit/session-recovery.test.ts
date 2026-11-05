import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SessionResponse } from "@/app/lib/api/types";

function json(body: unknown, status = 200) { return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } }); }
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>((done) => { resolve = done; }); return { promise, resolve }; }
const oldSession: SessionResponse = { authenticated: true, csrfToken: "old-csrf", user: { id: "old-user", roles: ["guardian"] } };
const newSession: SessionResponse = { authenticated: true, csrfToken: "new-csrf", user: { id: "new-user", roles: ["coach"] } };

beforeEach(() => { vi.resetModules(); process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.example.test"; });
afterEach(() => { vi.restoreAllMocks(); delete process.env.NEXT_PUBLIC_API_BASE_URL; });

describe("remembered session recovery", () => {
  it("deduplicates bootstrap and restores with device CSRF", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(json({ authenticated: false, restoreAvailable: true, csrfToken: "device-csrf" }))
      .mockResolvedValueOnce(json(newSession));
    const auth = await import("@/app/lib/auth-client");
    const first = auth.getSession(); const second = auth.getSession();
    expect(first).toBe(second);
    await expect(first).resolves.toEqual(newSession);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    const restore = fetchMock.mock.calls[1];
    expect(restore[0]).toBe("https://api.example.test/api/v1/auth/session/restore");
    expect(new Headers((restore[1] as RequestInit).headers).get("X-CSRF-Token")).toBe("device-csrf");
    expect((restore[1] as RequestInit).credentials).toBe("include");
  });

  it("retains authenticated state when bootstrap fails transiently", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(json(oldSession))
      .mockResolvedValueOnce(json({ status: 503, code: "session_unavailable", title: "Session service unavailable" }, 503));
    const auth = await import("@/app/lib/auth-client");
    const state = await import("@/app/lib/session-state");
    await auth.getSession();
    await expect(auth.getSession()).rejects.toMatchObject({ status: 503 });
    expect(state.readSessionState().session).toEqual(oldSession);
    expect(state.readSessionState().error).toContain("Session service unavailable");
  });

  it("does not make a restore request for a confirmed signed-out browser", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(json({ authenticated: false }));
    const auth = await import("@/app/lib/auth-client");
    await expect(auth.getSession()).resolves.toEqual({ authenticated: false });
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("does not return an obsolete identity after a newer login completes", async () => {
    const stale = deferred<Response>();
    vi.spyOn(globalThis, "fetch").mockImplementation((request) => String(request instanceof Request ? request.url : request).endsWith("/auth/session") ? stale.promise : Promise.resolve(json(newSession)));
    const auth = await import("@/app/lib/auth-client");
    const api = await import("@/app/lib/api/client");
    const state = await import("@/app/lib/session-state");
    const previous = auth.getSession();
    await auth.signInWithPassword("new-user", "test-password");
    stale.resolve(json(oldSession));
    await expect(previous).resolves.toEqual(newSession);
    expect(state.readSessionState().session).toEqual(newSession);
    expect(api.getCSRFToken()).toBe("new-csrf");
  });

  it("does not restore stale state after an explicit logout from another tab", async () => {
    const stale = deferred<Response>();
    vi.spyOn(globalThis, "fetch").mockReturnValue(stale.promise);
    const auth = await import("@/app/lib/auth-client");
    const state = await import("@/app/lib/session-state");
    const pending = auth.getSession();
    auth.clearSession();
    stale.resolve(json(oldSession));
    await expect(pending).resolves.toEqual({ authenticated: false });
    expect(state.readSessionState().session).toEqual({ authenticated: false });
  });

  it("does not overwrite a new login's CSRF with a stale MFA response", async () => {
    const stale = deferred<Response>();
    vi.spyOn(globalThis, "fetch").mockImplementation((request) => String(request instanceof Request ? request.url : request).endsWith("/auth/mfa/status") ? stale.promise : Promise.resolve(json(newSession)));
    const auth = await import("@/app/lib/auth-client");
    const api = await import("@/app/lib/api/client");
    const previous = auth.getMFAStatus();
    await auth.signInWithPassword("new-user", "test-password");
    stale.resolve(json({ required: true, enrolled: true, verifiedThisSession: false, csrfToken: "stale-mfa" }));
    await previous;
    expect(api.getCSRFToken()).toBe("new-csrf");
  });
});

describe("authorized API retries", () => {
  it("refreshes an expired session once and preserves the original mutation", async () => {
    const api = await import("@/app/lib/api/client");
    const recover = vi.fn(async () => { api.setCSRFToken("renewed-csrf"); return true; });
    api.setSessionRecovery(recover);
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(json({ code: "unauthenticated" }, 401)).mockResolvedValueOnce(new Response(null, { status: 204 }));
    const response = await api.authenticatedFetch(new Request("https://api.example.test/api/v1/classes", { method: "POST", credentials: "include", headers: { "X-CSRF-Token": "old-csrf", "Content-Type": "application/json" }, body: JSON.stringify({ title: "Endgames" }) }));
    expect(response.status).toBe(204); expect(recover).toHaveBeenCalledOnce(); expect(fetchMock).toHaveBeenCalledTimes(2);
    const retry = fetchMock.mock.calls[1][0] as Request;
    expect(retry.headers.get("X-CSRF-Token")).toBe("renewed-csrf");
    expect(retry.credentials).toBe("include");
    expect(await retry.json()).toEqual({ title: "Endgames" });
  });

  it.each([[403, "forbidden"], [403, "recent_mfa_required"], [409, "conflict"], [503, "unavailable"]])("does not retry a %s %s rejection", async (status, code) => {
    const api = await import("@/app/lib/api/client"); const recover = vi.fn(async () => true); api.setSessionRecovery(recover);
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(json({ code }, status as number));
    const response = await api.authenticatedFetch(new Request("https://api.example.test/api/v1/classes"));
    expect(response.status).toBe(status); expect(fetchMock).toHaveBeenCalledOnce(); expect(recover).not.toHaveBeenCalled();
  });

  it("refreshes explicit stale CSRF once", async () => {
    const api = await import("@/app/lib/api/client"); const recover = vi.fn(async () => true); api.setSessionRecovery(recover);
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(json({ code: "invalid_csrf" }, 403));
    const response = await api.authenticatedFetch(new Request("https://api.example.test/api/v1/classes", { method: "POST" }));
    expect(response.status).toBe(403); expect(fetchMock).toHaveBeenCalledTimes(2); expect(recover).toHaveBeenCalledOnce();
  });
});
