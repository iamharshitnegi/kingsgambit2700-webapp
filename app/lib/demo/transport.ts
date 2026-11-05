import { demoAcademyRequest } from "./academy";
import { demoLearningRequest } from "./learning";
import { fallbackCurriculum, fallbackOrganization, fallbackPolicies, fallbackPrograms } from "../fallback-content";
import type { SupportCase } from "../api/typed-client";
import type { TrialFormat } from "../api/types";
import { demoAccounts, type DemoWorkspace } from "./data";
import { applyDemoAction, DemoError, getDemoUser, loginDemo, logoutDemo, readDemo, requireRole, workspaceForUser, writeDemo } from "./store";
import { isDemoMode } from "./mode";

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json", "Cache-Control": "no-store" } });
}

function textField(body: Record<string, unknown>, field: string, max = 10000) {
  const value = typeof body[field] === "string" ? body[field].trim() : "";
  if (!value || value.length > max) throw new DemoError(`Please enter a valid ${field}.`, 422);
  return value;
}

function slots(state: DemoWorkspace, format: TrialFormat) {
  // Stable daily IDs make refreshes and repeat submissions address the same slots.
  const base = new Date(); base.setUTCHours(10, 30, 0, 0);
  return Array.from({ length: 6 }, (_, day) => {
    const start = new Date(base.getTime() + (day + 1) * 86_400_000);
    const id = `demo-${format}-${start.toISOString().slice(0, 10)}`;
    return { id, startsAt: start.toISOString(), endsAt: new Date(start.getTime() + 30 * 60_000).toISOString(), format, ageBands: ["4_7", "8_12", "13_17", "18_plus"], availableSeats: (format === "group" ? 6 : 1) - state.bookings.filter((booking) => booking.slotId === id && booking.status === "confirmed").length, coachDisplayName: "Coach Maya · sample profile" };
  });
}

function publicPolicy(slug: string) {
  const policy = fallbackPolicies[slug];
  if (!policy) throw new DemoError("Policy not found.", 404);
  return { ...policy, version: "demo-1", purpose: policy.summary, contentHtml: policy.contentHtml || policy.sections?.map((section) => `<h2>${section.heading}</h2><p>${section.body}</p>`).join("") || "<p>Sample policy for local testing only.</p>" };
}

function support(path: string, method: string, url: URL, body: Record<string, unknown>, state: DemoWorkspace) {
  const user = requireRole("guardian", "operations_admin", "super_admin");
  const manager = user.roles.some((role) => ["operations_admin", "super_admin"].includes(role));
  const visible = (item: SupportCase): SupportCase => ({ ...item, events: manager ? item.events : item.events.filter((event) => event.visibility === "customer_visible") });
  const familyId = method === "GET" ? url.searchParams.get("familyId") : body.familyId;
  if (path === "/support/cases") {
    if (familyId !== state.family.id) throw new DemoError("This family record is not assigned to you.", 403);
    if (method === "GET") return json({ items: state.supportCases.map(visible) });
    if (method !== "POST") throw new DemoError("Method not allowed.", 405);
    const now = new Date().toISOString();
    if (!["class", "learning", "account", "policy", "technical", "other"].includes(String(body.category)) || !["low", "medium", "high", "critical"].includes(String(body.severity))) throw new DemoError("Choose a category and priority.", 422);
    const item: SupportCase = { id: crypto.randomUUID(), familyId: state.family.id, openedBy: user.id, category: body.category as SupportCase["category"], severity: body.severity as SupportCase["severity"], status: "open", subject: textField(body, "subject", 200), openedAt: now, updatedAt: now, slaDueAt: new Date(Date.now() + 86_400_000).toISOString(), version: 1, events: [{ id: crypto.randomUUID(), type: "opened", visibility: "customer_visible", actorId: user.id, body: textField(body, "message"), metadata: {}, occurredAt: now }] };
    state.supportCases.unshift(item);
    state.audit.unshift({ id: crypto.randomUUID(), actor: user.displayName || user.id, action: "Opened support case", at: now });
    writeDemo(state);
    return json(visible(item), 201);
  }
  const [, , , id, operation] = path.split("/");
  const item = state.supportCases.find((entry) => entry.id === id);
  if (!item) throw new DemoError("Support case not found.", 404);
  if (method === "GET" && !operation) return json(visible(item));
  if (body.version !== item.version) throw new DemoError("This case has changed. Reopen it before making another change.", 409);
  if (!manager && (operation !== "events" || body.visibility === "internal")) throw new DemoError("Only academy staff may make this change.", 403);
  const now = new Date().toISOString();
  if (operation === "events" && method === "POST") {
    if (!["internal", "customer_visible"].includes(String(body.visibility))) throw new DemoError("Choose a valid reply visibility.", 422);
    item.events.push({ id: crypto.randomUUID(), type: "message", visibility: body.visibility as "internal" | "customer_visible", actorId: user.id, body: textField(body, "body"), metadata: {}, occurredAt: now });
  } else if (operation === "status" && method === "PATCH") {
    const transitions: Record<SupportCase["status"], string[]> = { open: ["investigating", "waiting", "resolved"], investigating: ["waiting", "resolved"], waiting: ["investigating", "resolved"], resolved: ["investigating", "closed"], closed: ["investigating"] };
    if (!transitions[item.status].includes(String(body.status))) throw new DemoError("That status transition is not available.", 409);
    item.status = body.status as SupportCase["status"];
    item.events.push({ id: crypto.randomUUID(), type: "status_changed", visibility: "customer_visible", actorId: user.id, body: textField(body, "reason", 500), metadata: { status: item.status }, occurredAt: now });
  } else if (operation === "assignment" && method === "PATCH") {
    item.assignedTo = textField(body, "assigneeId", 100);
    item.events.push({ id: crypto.randomUUID(), type: "assigned", visibility: "internal", actorId: user.id, body: textField(body, "reason", 500), metadata: {}, occurredAt: now });
  } else throw new DemoError("That support operation is not available.", 404);
  item.version += 1; item.updatedAt = now;
  state.audit.unshift({ id: crypto.randomUUID(), actor: user.displayName || user.id, action: `Support ${operation}`, at: now });
  writeDemo(state);
  return json(visible(item));
}

/** Local transport shares the real API clients, so forms exercise the same request/error path. */
export async function demoFetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  if (!isDemoMode()) return json({ title: "Demo disabled", status: 404 }, 404);
  try {
    const request = input instanceof Request ? input : new Request(input, init);
    const url = new URL(request.url);
    const path = url.pathname.replace(/^\/api\/v1/, "");
    const method = request.method;
    const body = ["GET", "HEAD"].includes(method) || request.headers.get("Content-Type")?.includes("multipart/form-data") ? {} : await request.json().catch(() => ({})) as Record<string, unknown>;
    const state = readDemo();
    if (path === "/auth/session" && method === "GET") { const user = getDemoUser(); return json({ authenticated: !!user, user, csrfToken: "local-demo-csrf" }); }
    if (path === "/auth/logout" && method === "POST") { logoutDemo(); return new Response(null, { status: 204 }); }
    if (path === "/auth/magic-links" && method === "POST") {
      const account = demoAccounts.find((account) => account.email === String(body.email).trim().toLowerCase());
      if (!account) throw new DemoError("Use one of the sample account emails shown on the Local demo page.", 422);
      loginDemo(account.role);
      return json({ status: "accepted" }, 202);
    }
    if (path === "/auth/student-codes/exchange" && method === "POST") {
      if (body.code !== "DEMO2700") throw new DemoError("The local demo student code is DEMO2700.", 422);
      return json(loginDemo("student"));
    }
    if (path === "/me" && method === "GET") { const user = requireRole(...demoAccounts.map((account) => account.role)); return json({ user, familyIds: [state.family.id] }); }
    if (path === "/demo/workspace" && method === "GET") return json(workspaceForUser(state, url.searchParams.get("classId") || undefined));
    if (path === "/demo/actions" && method === "POST") return json(applyDemoAction(body));
    if (path.startsWith("/academy/")) { const result = await demoAcademyRequest(path, method, body, state, request); if (result) return result; }
    if (path.startsWith("/learning/")) { const result = await demoLearningRequest(path, method, body, state, request); if (result) return result; }
    if (path.startsWith("/support/cases")) return support(path, method, url, body, state);
    if (path === "/public/organization" && method === "GET") return json({ name: fallbackOrganization.name, email: fallbackOrganization.contactEmail, whatsApp: fallbackOrganization.whatsappNumber, canonicalUrl: fallbackOrganization.siteUrl, socialLinks: { instagram: fallbackOrganization.instagramUrl } });
    if (path === "/public/programs" && method === "GET") return json({ items: fallbackPrograms });
    if (path === "/public/curriculum" && method === "GET") return json(fallbackCurriculum);
    if (path === "/public/coaches" && method === "GET") return json({ items: [{ id: "demo-coach", slug: "maya", displayName: "Coach Maya · Demo", bio: "A fictional coaching profile for exploring your local academy. Enjoys helping young players explain the thinking behind each move.", languages: ["English", "Hindi"] }] });
    if (path === "/public/pricing" && method === "GET") return json({ currency: "INR", items: [{ planId: "demo-group", programSlug: "small-group", name: "Small-group academy · Sample", format: "group", billingModel: "class_pack", amountMinor: 320000, classQuantity: 8, capacity: 6, taxInclusive: true, features: ["8 live lessons", "Practice and feedback", "Monthly progress report", "Sample price for local testing"] }, { planId: "demo-private", programSlug: "one-to-one", name: "Individual coaching · Sample", format: "one_to_one", billingModel: "class_pack", amountMinor: 640000, classQuantity: 8, capacity: 1, taxInclusive: true, features: ["8 individual lessons", "Personalised practice", "Sample price for local testing"] }] });
    if (path === "/public/policies" && method === "GET") return json({ items: Object.keys(fallbackPolicies).map(publicPolicy) });
    if (path.startsWith("/public/policies/") && method === "GET") return json(publicPolicy(path.split("/")[3]));
    if (path === "/public/trial-slots" && method === "GET") {
      const format = url.searchParams.get("format");
      if (format !== "group" && format !== "one_to_one") throw new DemoError("Choose a trial format.", 422);
      const from = Date.parse(url.searchParams.get("from") || ""); const to = Date.parse(url.searchParams.get("to") || "");
      return json({ items: slots(state, format).filter((slot) => slot.availableSeats > 0 && (!Number.isFinite(from) || Date.parse(slot.startsAt) >= from) && (!Number.isFinite(to) || Date.parse(slot.startsAt) <= to)) });
    }
    if (path === "/public/leads" && method === "POST") {
      const guardianName = textField(body, "guardianName", 100);
      if (body.contactConsent !== true) throw new DemoError("Contact consent is required.", 422);
      const email = String(body.email || "").trim().toLowerCase(); const phone = String(body.phone || "").trim();
      if ((!email && !phone) || (email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email))) throw new DemoError("Enter a valid contact email or phone number.", 422);
      if (!["4_7", "8_12", "13_17", "18_plus"].includes(String(body.studentAgeBand))) throw new DemoError("Choose a student age band.", 422);
      const existing = state.leads.find((lead) => email ? lead.email === email : lead.phone === phone);
      if (existing) return json(existing);
      const lead = { id: crypto.randomUUID(), guardianName, email, phone, studentAgeBand: String(body.studentAgeBand), status: "new", createdAt: new Date().toISOString() };
      state.leads.unshift(lead); writeDemo(state); return json(lead, 201);
    }
    if (path === "/public/trial-bookings" && method === "POST") {
      const key = request.headers.get("Idempotency-Key");
      if (!key) throw new DemoError("A booking request key is required.", 422);
      const existing = state.bookings.find((booking) => booking.idempotencyKey === key);
      if (existing) {
        if (existing.slotId !== body.slotId || existing.leadId !== body.leadId || existing.email !== body.email || existing.timeZone !== body.timeZone) throw new DemoError("This request key was used for different booking details.", 409);
        return json(existing);
      }
      if (body.contactConsent !== true || !state.leads.some((lead) => lead.id === body.leadId)) throw new DemoError("Save the enquiry and agree to contact before booking.", 422);
      const slot = [...slots(state, "group"), ...slots(state, "one_to_one")].find((slot) => slot.id === body.slotId);
      if (!slot || slot.availableSeats < 1) throw new DemoError("That slot is no longer available. Please choose another time.", 409);
      const booking = { id: crypto.randomUUID(), leadId: String(body.leadId), slotId: slot.id, guardianName: textField(body, "guardianName", 100), email: textField(body, "email", 320), startsAt: slot.startsAt, endsAt: slot.endsAt, timeZone: textField(body, "timeZone", 100), status: "confirmed", idempotencyKey: key };
      state.bookings.push(booking);
      state.leads.find((lead) => lead.id === body.leadId)!.status = "trial_booked";
      state.audit.unshift({ id: crypto.randomUUID(), actor: booking.guardianName, action: "Trial booked", at: new Date().toISOString() });
      writeDemo(state); return json(booking, 201);
    }
    throw new DemoError("This workflow is not available in the local demo.", 404);
  } catch (error) {
    const status = error instanceof DemoError ? error.status : 500;
    return json({ title: "Request could not be completed", detail: error instanceof Error ? error.message : "Please retry.", status }, status);
  }
}
