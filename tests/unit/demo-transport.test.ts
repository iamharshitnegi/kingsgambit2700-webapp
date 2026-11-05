import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SupportCase } from "@/app/lib/api/typed-client";
import type { TrialSlot } from "@/app/lib/api/types";
import { DEMO_FAMILY_ID, type DemoBooking, type DemoLead, type DemoWorkspace } from "@/app/lib/demo/data";
import { demoFetch } from "@/app/lib/demo/transport";
import { getDemoUser, loginDemo, readDemo } from "@/app/lib/demo/store";

beforeEach(() => {
  vi.stubEnv("NEXT_PUBLIC_DEMO_MODE", "true");
  vi.stubEnv("NODE_ENV", "test");
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date("2026-09-08T08:00:00Z"));
  localStorage.clear();
  sessionStorage.clear();
});

afterEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  vi.useRealTimers();
  vi.unstubAllEnvs();
});

function request(path: string, method = "GET", body?: Record<string, unknown>, key?: string) {
  return demoFetch(`http://localhost:3000/api/v1${path}`, {
    method,
    headers: { "Content-Type": "application/json", ...(key ? { "Idempotency-Key": key } : {}) },
    ...(body ? { body: JSON.stringify(body) } : {}),
  });
}

async function createLead(index = 0) {
  const response = await request("/public/leads", "POST", {
    guardianName: `Guardian ${index}`,
    email: `guardian${index}@example.test`,
    studentAgeBand: "8_12",
    country: "IN",
    timeZone: "Asia/Kolkata",
    contactConsent: true,
  });
  expect(response.status).toBe(201);
  return await response.json() as DemoLead;
}

async function availableSlots(format: "group" | "one_to_one" = "group") {
  const response = await request(`/public/trial-slots?format=${format}&timezone=Asia%2FKolkata`);
  expect(response.status).toBe(200);
  return (await response.json() as { items: TrialSlot[] }).items;
}

function bookingInput(lead: DemoLead, slot: TrialSlot) {
  return { leadId: lead.id, slotId: slot.id, guardianName: lead.guardianName, email: lead.email, studentAgeBand: "8_12", timeZone: "Asia/Kolkata", contactConsent: true };
}

async function openSupportCase() {
  loginDemo("guardian");
  const response = await request("/support/cases", "POST", { familyId: DEMO_FAMILY_ID, category: "class", severity: "medium", subject: "Help with a class", message: "Can we discuss a suitable make-up lesson?" });
  expect(response.status).toBe(201);
  return await response.json() as SupportCase;
}

describe("mock HTTP authentication and safety boundary", () => {
  it("blocks the complete mock transport in production even when the flag is set", async () => {
    vi.stubEnv("NODE_ENV", "production");
    const response = await request("/auth/magic-links", "POST", { email: "admin@example.test" });
    expect(response.status).toBe(404);
    expect(await response.json()).toMatchObject({ status: 404 });
    expect(localStorage.length).toBe(0);
    expect(sessionStorage.length).toBe(0);
  });

  it("returns problem details for unauthenticated workspaces and rejects unknown accounts", async () => {
    const response = await request("/demo/workspace");
    expect(response.status).toBe(401);
    expect(response.headers.get("cache-control")).toBe("no-store");
    expect(await response.json()).toMatchObject({ status: 401, detail: expect.stringMatching(/sign in/i) });
    const unknown = await request("/auth/magic-links", "POST", { email: "someone@example.test" });
    expect(unknown.status).toBe(422);
    expect(getDemoUser()).toBeUndefined();
  });

  it("opens and revokes local sessions through the existing auth endpoints", async () => {
    const login = await request("/auth/magic-links", "POST", { email: "GUARDIAN@example.test" });
    expect(login.status).toBe(202);
    const session = await request("/auth/session");
    expect(await session.json()).toMatchObject({ authenticated: true, user: { roles: ["guardian"] } });
    expect((await request("/auth/logout", "POST")).status).toBe(204);
    expect(await (await request("/auth/session")).json()).toMatchObject({ authenticated: false });
    expect((await request("/auth/student-codes/exchange", "POST", { code: "wrong-code" })).status).toBe(422);
    const student = await request("/auth/student-codes/exchange", "POST", { code: "DEMO2700" });
    expect(student.status).toBe(200);
    expect(await student.json()).toMatchObject({ authenticated: true, user: { roles: ["student"] } });
  });
});

describe("mock enquiry and trial booking lifecycle", () => {
  it("requires consent and contact information and deduplicates repeated enquiries", async () => {
    const input = { guardianName: "Test Guardian", email: "test@example.test", studentAgeBand: "8_12", contactConsent: false };
    expect((await request("/public/leads", "POST", input)).status).toBe(422);
    expect((await request("/public/leads", "POST", { ...input, contactConsent: true, email: "invalid" })).status).toBe(422);
    const before = readDemo().leads.length;
    const created = await request("/public/leads", "POST", { ...input, contactConsent: true });
    expect(created.status).toBe(201);
    const first = await created.json() as DemoLead;
    const repeated = await request("/public/leads", "POST", { ...input, contactConsent: true, email: "TEST@example.test" });
    expect(repeated.status).toBe(200);
    expect(await repeated.json()).toMatchObject({ id: first.id });
    expect(readDemo().leads).toHaveLength(before + 1);
  });

  it("requires a saved lead, consent and an idempotency key before booking", async () => {
    const lead = await createLead();
    const slot = (await availableSlots())[0];
    const input = bookingInput(lead, slot);
    expect((await request("/public/trial-bookings", "POST", input)).status).toBe(422);
    expect((await request("/public/trial-bookings", "POST", { ...input, leadId: "unknown-lead" }, "unknown-lead-key")).status).toBe(422);
    expect((await request("/public/trial-bookings", "POST", { ...input, contactConsent: false }, "missing-consent-key")).status).toBe(422);
    expect(readDemo().bookings).toEqual([]);
  });

  it("replays a booking without duplicate records and rejects reuse for different details", async () => {
    const lead = await createLead();
    const slots = await availableSlots();
    const input = bookingInput(lead, slots[0]);
    const key = "stable-booking-request";
    const first = await request("/public/trial-bookings", "POST", input, key);
    expect(first.status).toBe(201);
    const booking = await first.json() as DemoBooking;
    const afterBooking = readDemo();
    const repeated = await request("/public/trial-bookings", "POST", input, key);
    expect(repeated.status).toBe(200);
    expect(await repeated.json()).toEqual(booking);
    expect(readDemo()).toEqual(afterBooking);
    const changed = await request("/public/trial-bookings", "POST", { ...input, slotId: slots[1].id }, key);
    expect(changed.status).toBe(409);
    expect(readDemo()).toEqual(afterBooking);
    expect(afterBooking.leads.find((item) => item.id === lead.id)?.status).toBe("trial_booked");
  });

  it.each([{ format: "group", seats: 6 }, { format: "one_to_one", seats: 1 }] as const)("enforces $seats seats for $format while allowing a replay after capacity is full", async ({ format, seats }) => {
    const slot = (await availableSlots(format))[0];
    const firstLead = await createLead(0);
    const firstInput = bookingInput(firstLead, slot);
    for (let index = 0; index < seats; index += 1) {
      const lead = index === 0 ? firstLead : await createLead(index);
      expect((await request("/public/trial-bookings", "POST", bookingInput(lead, slot), `seat-${index}`)).status).toBe(201);
    }
    const fullState = readDemo();
    expect(fullState.bookings.filter((booking) => booking.slotId === slot.id)).toHaveLength(seats);
    const overflowLead = await createLead(seats);
    expect((await request("/public/trial-bookings", "POST", bookingInput(overflowLead, slot), "overflow-request")).status).toBe(409);
    expect((await availableSlots(format)).some((item) => item.id === slot.id)).toBe(false);
    expect((await request("/public/trial-bookings", "POST", firstInput, "seat-0")).status).toBe(200);
    expect(readDemo().bookings).toHaveLength(seats);
  });
});

describe("mock family support permissions and concurrency", () => {
  it("hides staff notes in the guardian case detail, list and workspace", async () => {
    const supportCase = await openSupportCase();
    loginDemo("operations_admin");
    const internal = await request(`/support/cases/${supportCase.id}/events`, "POST", { version: supportCase.version, visibility: "internal", body: "Staff-only scheduling note." });
    expect(internal.status).toBe(200);
    const updated = await internal.json() as SupportCase;
    expect(updated.events.some((event) => event.body === "Staff-only scheduling note.")).toBe(true);
    loginDemo("guardian");
    const detail = await (await request(`/support/cases/${supportCase.id}`)).json() as SupportCase;
    const list = await (await request(`/support/cases?familyId=${DEMO_FAMILY_ID}`)).json() as { items: SupportCase[] };
    const workspace = await (await request("/demo/workspace")).json() as DemoWorkspace;
    for (const view of [detail, list.items[0], workspace.supportCases[0]]) {
      expect(view.events.every((event) => event.visibility === "customer_visible")).toBe(true);
      expect(view.events.some((event) => event.body === "Staff-only scheduling note.")).toBe(false);
    }
    expect(readDemo().supportCases[0].events).toHaveLength(2);
  });

  it("rejects stale updates and permits a reply after reloading the latest version", async () => {
    const supportCase = await openSupportCase();
    const first = await request(`/support/cases/${supportCase.id}/events`, "POST", { version: supportCase.version, visibility: "customer_visible", body: "One more detail about our schedule." });
    expect(first.status).toBe(200);
    const updated = await first.json() as SupportCase;
    const saved = readDemo();
    const stale = await request(`/support/cases/${supportCase.id}/events`, "POST", { version: supportCase.version, visibility: "customer_visible", body: "Stale tab reply." });
    expect(stale.status).toBe(409);
    expect(readDemo()).toEqual(saved);
    const fresh = await request(`/support/cases/${supportCase.id}/events`, "POST", { version: updated.version, visibility: "customer_visible", body: "Reply after refreshing." });
    expect(fresh.status).toBe(200);
    expect((await fresh.json() as SupportCase).version).toBe(supportCase.version + 2);
  });

  it("prevents guardians from writing internal notes or managing status and assignment", async () => {
    const supportCase = await openSupportCase();
    const saved = readDemo();
    const inputs = [
      { operation: "events", method: "POST", body: { visibility: "internal", body: "Unauthorized note." } },
      { operation: "status", method: "PATCH", body: { status: "resolved", reason: "Unauthorized resolution." } },
      { operation: "assignment", method: "PATCH", body: { assigneeId: "demo-super_admin", reason: "Unauthorized assignment." } },
    ];
    for (const input of inputs) {
      const response = await request(`/support/cases/${supportCase.id}/${input.operation}`, input.method, { version: supportCase.version, ...input.body });
      expect(response.status).toBe(403);
    }
    expect((await request("/support/cases?familyId=another-family")).status).toBe(403);
    expect(readDemo()).toEqual(saved);
    loginDemo("student");
    expect((await request(`/support/cases/${supportCase.id}`)).status).toBe(403);
  });

  it("records staff resolution and rejects unsupported status transitions", async () => {
    const supportCase = await openSupportCase();
    loginDemo("operations_admin");
    const invalid = await request(`/support/cases/${supportCase.id}/status`, "PATCH", { version: supportCase.version, status: "closed", reason: "Skip resolution." });
    expect(invalid.status).toBe(409);
    const resolved = await request(`/support/cases/${supportCase.id}/status`, "PATCH", { version: supportCase.version, status: "resolved", reason: "The replacement class is confirmed." });
    expect(resolved.status).toBe(200);
    const result = await resolved.json() as SupportCase;
    expect(result.status).toBe("resolved");
    expect(result.events.at(-1)).toMatchObject({ type: "status_changed", visibility: "customer_visible", body: "The replacement class is confirmed." });
  });
});
