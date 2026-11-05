import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

test("home page communicates the academy path and passes an accessibility scan", async ({ page }) => {
  // Scan the settled page instead of sampling the hero's opacity animation.
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).toContainText("From beginner to grandmaster");
  await expect(page.getByRole("link", { name: /Book a free trial/ }).first()).toBeVisible();
  const results = await new AxeBuilder({ page }).analyze();
  expect(results.violations).toEqual([]);
});

test("trial booking persists the lead before confirming the slot", async ({ page }) => {
  const startsAt = new Date(Date.now() + 3 * 24 * 60 * 60 * 1000).toISOString();
  const endsAt = new Date(Date.now() + 3 * 24 * 60 * 60 * 1000 + 30 * 60 * 1000).toISOString();
  const calls: string[] = [];
  let leadPayload: Record<string, unknown> = {};
  let idempotencyKey = "";

  await page.route("**/api/v1/public/trial-slots**", async (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ items: [{ id: "slot-1", startsAt, endsAt, timeZone: "Asia/Kolkata", format: "group", ageBands: ["8_12"], levelSlugs: [], availableSeats: 1 }] }) }));
  await page.route("**/api/v1/public/leads", async (route) => { calls.push("lead"); leadPayload = route.request().postDataJSON(); await route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify({ id: "lead-1", status: "new", createdAt: new Date().toISOString() }) }); });
  await page.route("**/api/v1/public/trial-bookings", async (route) => { calls.push("booking"); idempotencyKey = route.request().headers()["idempotency-key"] || ""; await route.fulfill({ status: 201, contentType: "application/json", body: JSON.stringify({ id: "booking-1", status: "confirmed", startsAt, endsAt, timeZone: "Asia/Kolkata" }) }); });

  await page.goto("/free-trial");
  await page.getByLabel("Guardian or adult learner name").fill("Test Guardian");
  await page.getByLabel("Student age").selectOption("8_12");
  await page.getByLabel("Two-letter country code").fill("in");
  await page.locator('input[name="slotId"]').first().check();
  await page.getByLabel("Email").fill("guardian@example.test");
  await page.getByLabel("I agree that the academy may contact me to arrange and deliver this trial.").check();
  await page.getByRole("button", { name: "Confirm trial" }).click();

  await expect(page.getByRole("heading", { name: "Your assessment is booked." })).toBeVisible();
  expect(calls).toEqual(["lead", "booking"]);
  expect(leadPayload).toMatchObject({ studentAgeBand: "8_12", preferredFormat: "group", country: "IN" });
  expect(idempotencyKey.length).toBeGreaterThanOrEqual(16);
});
