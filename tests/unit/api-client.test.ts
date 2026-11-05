import { afterEach, describe, expect, it, vi } from "vitest";
import { APIError, apiFetch, getAPIBaseURL, setCSRFToken } from "@/app/lib/api/client";

afterEach(() => {
  delete process.env.API_BASE_URL;
  delete process.env.NEXT_PUBLIC_API_BASE_URL;
  delete process.env.NEXT_PUBLIC_SITE_URL;
  setCSRFToken(undefined);
});

describe("academy API client", () => {
  it("uses the configured API origin and sends credentials", async () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.example.test";
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), { status: 200, headers: { "Content-Type": "application/json" } }),
    );

    await apiFetch("/public/organization");
    expect(getAPIBaseURL()).toBe("https://api.example.test/api/v1");
    expect(fetchMock).toHaveBeenCalledWith(
      "https://api.example.test/api/v1/public/organization",
      expect.objectContaining({ credentials: "include", method: "GET" }),
    );
  });

  it("rejects an API origin with an explicit trailing slash", () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.example.test/";
    expect(() => getAPIBaseURL()).toThrow(/must not have a trailing slash/);
  });

  it("adds the current CSRF token to state-changing requests", async () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.example.test";
    setCSRFToken("csrf-test");
    const fetchMock = vi.spyOn(globalThis, "fetch").mockResolvedValue(new Response(null, { status: 204 }));

    await apiFetch("/auth/logout", { method: "POST" });
    const request = fetchMock.mock.calls[0][1] as RequestInit;
    expect(new Headers(request.headers).get("X-CSRF-Token")).toBe("csrf-test");
  });

  it("preserves field-level problem details", async () => {
    process.env.NEXT_PUBLIC_API_BASE_URL = "https://api.example.test";
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ title: "Invalid request", status: 422, code: "validation_failed", errors: [{ field: "email", message: "Enter a valid email." }] }), { status: 422, headers: { "Content-Type": "application/problem+json" } }),
    );

    await expect(apiFetch("/public/leads", { method: "POST", body: "{}" })).rejects.toMatchObject({ status: 422, problem: { title: "Invalid request", status: 422, code: "validation_failed" } } satisfies Partial<APIError>);
  });
});
