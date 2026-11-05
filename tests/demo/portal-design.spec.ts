import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

test("saved class details, meeting link, calendar views and mobile navigation", async ({
  page,
}, info) => {
  test.setTimeout(90000);
  if (info.project.name.includes("mobile")) await page.setViewportSize({ width: 320, height: 800 });
  await page.goto("/demo");
  await page.getByRole("button", { name: /Coach Maya/ }).click();
  await expect(page).toHaveURL(/\/coach$/);
  await page.goto("/coach/calendar");
  await page
    .getByRole("button", { name: "Schedule class", exact: true })
    .click();
  const dialog = page.getByRole("dialog", { name: "Schedule a class" });
  const day = new Date(Date.now() + 4 * 86400000).toISOString().slice(0, 10);
  await dialog
    .getByLabel("Lesson title or topic")
    .fill("Planning with purpose");
  await dialog.getByLabel("Local start").fill(`${day}T11:00`);
  await dialog
    .getByLabel("Zoom or Meet link")
    .fill("https://meet.google.com/abc-defg-hij");
  await dialog
    .getByRole("button", { name: "Preview schedule", exact: true })
    .click();
  await expect(
    dialog.getByRole("heading", { name: "1 class ready to schedule" }),
  ).toBeVisible();
  await dialog.getByRole("button", { name: "Back to details" }).click();
  await expect(dialog.getByLabel("Lesson title or topic")).toHaveValue(
    "Planning with purpose",
  );
  await dialog
    .getByRole("button", { name: "Preview schedule", exact: true })
    .click();
  await dialog
    .getByRole("button", { name: "Confirm schedule", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Planning with purpose", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Open meeting", exact: true }),
  ).toHaveAttribute("href", "https://meet.google.com/abc-defg-hij");
  await expect(
    page.getByRole("button", { name: "Join classroom", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "Learners 1", exact: true }).click();
  await expect(page.getByText("Attendance not yet recorded")).toBeVisible();
  await page.getByRole("button", { name: "Overview", exact: true }).click();
  await page.screenshot({
    path: info.outputPath("calendar.png"),
    fullPage: true,
  });
  expect((await new AxeBuilder({ page }).analyze()).violations).toEqual([]);
  await page
    .getByRole("button", { name: "Close class details", exact: true })
    .click();
  for (const mode of ["Day", "Week", "Month", "Agenda"]) {
    await page.getByRole("button", { name: mode, exact: true }).click();
    await expect(
      page.getByRole("button", { name: mode, exact: true }),
    ).toHaveAttribute("aria-pressed", "true");
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth + 1,
      ),
    ).toBe(true);
  }
  await page
    .getByRole("searchbox", { name: "Search classes" })
    .fill("no such lesson");
  await expect(page.getByText("No classes match these filters.")).toBeVisible();
  await page.getByRole("button", { name: "Clear filters" }).click();
  await page.goto("/coach/classes");
  await page
    .getByRole("button", { name: "View Planning with purpose", exact: true })
    .click();
  await expect(
    page.getByRole("link", { name: "Open meeting", exact: true }),
  ).toBeVisible();
  await page.reload();
  await page
    .getByRole("button", { name: "View Planning with purpose", exact: true })
    .click();
  await expect(
    page.getByRole("link", { name: "Open meeting", exact: true }),
  ).toHaveAttribute("href", "https://meet.google.com/abc-defg-hij");
  if (info.project.name.includes("mobile")) {
    await page.getByRole("button", { name: "Open portal navigation" }).click();
    const menu = page.getByRole("dialog", { name: "Academy navigation" });
    await expect(menu).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(menu).not.toBeVisible();
    await expect(
      page.getByRole("button", { name: "Open portal navigation" }),
    ).toBeFocused();
  }
});

test("the shared design fits every academy view", async ({ page }, info) => {
  test.setTimeout(120000);
  await page.goto("/demo");
  await page.getByRole("button", { name: /admin@example.test/ }).click();
  await expect(page).toHaveURL(/\/admin$/);
  const views = {
    calendar: "Calendar",
    classes: "Classes",
    students: "Learners & batches",
    homework: "Homework.",
    board: "Teaching board",
    library: "Resource library",
    games: "Game reviews",
    progress: "Progress & feedback",
    messages: "Messages",
    notifications: "Notifications",
    operations: "Trials & operations",
    settings: "Settings & access",
  };
  for (const [section, title] of Object.entries(views)) {
    await page.goto(`/admin/${section}`);
    await expect(
      page.getByRole("heading", { name: title, level: 1, exact: true }),
    ).toBeVisible({ timeout: 15000 });
    await expect(
      page.getByRole("searchbox", { name: section === "homework" ? "Search homework" : "Search your academy" }),
    ).toBeVisible();
    await expect(
      page.getByRole("alert").filter({ hasText: /failed|unavailable/i }),
    ).toHaveCount(0);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth + 1,
      ),
      `${section} overflows the viewport`,
    ).toBe(true);
    if (["students", "homework", "progress", "settings"].includes(section))
      await page.screenshot({
        path: info.outputPath(`${section}.png`),
        fullPage: true,
      });
  }
});
