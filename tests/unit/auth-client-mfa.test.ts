import { afterEach, describe, expect, it, vi } from "vitest";
import {
  beginTOTPEnrollment,
  completeMFAChallenge,
  confirmTOTPEnrollment,
  getMFAStatus,
} from "@/app/lib/auth-client";
import { getCSRFToken, setCSRFToken } from "@/app/lib/api/client";

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

afterEach(() => {
  delete process.env.NEXT_PUBLIC_API_BASE_URL;
  setCSRFToken(undefined);
  vi.restoreAllMocks();
});

describe("MFA auth client", () => {
  it("keeps CSRF state in memory and rotates it after enrollment", async () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.example.test";
    const storageSpy = vi.spyOn(Storage.prototype, "setItem");
    const fetchMock = vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(jsonResponse({
        required: true,
        enrolled: false,
        enrollmentPending: false,
        verifiedThisSession: false,
        csrfToken: "mfa-status-csrf",
      }))
      .mockResolvedValueOnce(jsonResponse({
        secret: "JBSWY3DPEHPK3PXP",
        otpauthUri: "otpauth://totp/KingsGambit2700:test?secret=JBSWY3DPEHPK3PXP",
        expiresAt: "2026-08-30T12:05:00Z",
      }, 201))
      .mockResolvedValueOnce(jsonResponse({
        authenticated: true,
        csrfToken: "enrollment-csrf",
        user: { id: "user-1", roles: ["coach"] },
        recoveryCodes: ["ABCD-EFGH-IJKL-MNOP-QRST-UVWX"],
      }));

    await getMFAStatus();
    expect(getCSRFToken()).toBe("mfa-status-csrf");
    await beginTOTPEnrollment();
    await confirmTOTPEnrollment("123456");
    expect(getCSRFToken()).toBe("enrollment-csrf");

    const enrollmentRequest = fetchMock.mock.calls[1][1] as RequestInit;
    const confirmationRequest = fetchMock.mock.calls[2][1] as RequestInit;
    expect(new Headers(enrollmentRequest.headers).get("X-CSRF-Token")).toBe("mfa-status-csrf");
    expect(new Headers(confirmationRequest.headers).get("X-CSRF-Token")).toBe("mfa-status-csrf");
    expect(confirmationRequest.body).toBe(JSON.stringify({ code: "123456" }));
    expect(storageSpy).not.toHaveBeenCalled();
  });

  it("rotates the in-memory CSRF token after a recovery challenge", async () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.example.test";
    setCSRFToken("primary-csrf");
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(jsonResponse({
      authenticated: true,
      csrfToken: "verified-csrf",
      user: { id: "user-1", roles: ["coach"] },
    }));

    await completeMFAChallenge({ recoveryCode: "ABCD-EFGH-IJKL-MNOP-QRST-UVWX" });

    expect(getCSRFToken()).toBe("verified-csrf");
    const request = fetchMock.mock.calls[0][1] as RequestInit;
    expect(new Headers(request.headers).get("X-CSRF-Token")).toBe("primary-csrf");
    expect(request.body).toBe(JSON.stringify({ recoveryCode: "ABCD-EFGH-IJKL-MNOP-QRST-UVWX" }));
  });
});
