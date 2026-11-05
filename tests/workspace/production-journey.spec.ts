import AxeBuilder from "@axe-core/playwright";
import { readFile, writeFile } from "node:fs/promises";
import { createHmac } from "node:crypto";
import { expect, test, type Page } from "@playwright/test";
import type { AcademyWorkspaceData, LearningWorkspace } from "../../app/lib/academy-types";

const origin = "http://localhost:3005";
const api = "http://localhost:8085/api/v1";
const mailbox = "http://localhost:8025/api/v1";

async function request<T>(page: Page, path: string, body?: unknown, method = body === undefined ? "GET" : "POST"): Promise<T> {
  const session = await (await page.request.get(`${api}/auth/session`)).json();
  const response = await page.request.fetch(`${api}${path}`, { method, headers: { Origin: origin, ...(session.csrfToken ? { "X-CSRF-Token": session.csrfToken } : {}) }, ...(body === undefined ? {} : { data: body }) });
  expect(response.ok(), `${method} ${path}: ${response.status()} ${response.ok() ? "" : await response.text()}`).toBeTruthy();
  return response.status() === 204 ? undefined as T : response.json();
}

function authenticatorCode(secret: string) {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567";
  const bits = [...secret.replace(/=|\s/g, "").toUpperCase()].map(character => alphabet.indexOf(character).toString(2).padStart(5, "0")).join("");
  const key = Buffer.from((bits.match(/.{8}/g) || []).map(byte => parseInt(byte, 2)));
  const counter = Buffer.alloc(8); counter.writeBigUInt64BE(BigInt(Math.floor(Date.now() / 30000)));
  const digest = createHmac("sha1", key).update(counter).digest();
  const offset = digest[digest.length - 1] & 15;
  return ((digest.readUInt32BE(offset) & 0x7fffffff) % 1000000).toString().padStart(6, "0");
}

async function signIn(page: Page, email: string, staff = false) {
  await page.goto("/sign-in");
  const result = await page.request.post(`${api}/auth/magic-links`, { headers: { Origin: origin }, data: { email, redirectPath: "/auth/complete" } });
  expect(result.ok(), `Magic link: ${result.status()} ${result.ok() ? "" : await result.text()}`).toBeTruthy();
  let link = "";
  await expect.poll(async () => {
    const list = await (await page.request.get(`${mailbox}/search?query=${encodeURIComponent(`to:${email}`)}`)).json();
    const message = list.messages?.[0];
    if (!message) return false;
    const detail = await (await page.request.get(`${mailbox}/message/${message.ID}`)).json();
    link = String(detail.Text || detail.HTML).match(/https?:\/\/[^\s"<>]+\/api\/v1\/auth\/magic-links\/consume[^\s"<>]*/)?.[0]?.replaceAll("&amp;", "&") || "";
    return !!link;
  }, { timeout: 20000, message: "local sign-in email delivered" }).toBe(true);
  await page.goto(link);
  if (staff) {
    const fixturePath = `/tmp/kg-workspace-mfa-${process.env.KG_BROWSER_SCHEMA || "port-8085"}.json`;
    const fixtures: Record<string,string> = await readFile(fixturePath,"utf8").then(JSON.parse).catch(()=>({}));
    await expect(page.getByRole("button", {name:/Set up authenticator|Verify and continue/})).toBeVisible();
    if (await page.getByRole("button", {name:"Set up authenticator",exact:true}).isVisible()) {
    await page.getByRole("button", { name: "Set up authenticator" }).click();
    const secret = await page.getByLabel("Authenticator setup secret").textContent();
    expect(secret).toBeTruthy();
    fixtures[email]=secret!; await writeFile(fixturePath,JSON.stringify(fixtures),{mode:0o600});
    await page.getByLabel("Six-digit authenticator code").fill(authenticatorCode(secret!));
    await page.getByRole("button", { name: "Confirm authenticator" }).click();
    await page.getByRole("checkbox", { name: "I saved all recovery codes in a secure place." }).check();
    await page.getByRole("button", { name: "Continue to academy portal" }).click();
    } else {
      expect(fixtures[email],"Test MFA fixture unavailable: restart with a fresh isolated schema").toBeTruthy();
      // Advance the test-only authenticator counter so a fast rerun cannot reuse a consumed code.
      const previousCounter = Math.floor(Date.now() / 30000);
      await expect.poll(() => Math.floor(Date.now() / 30000) > previousCounter, {timeout:35000, intervals:[500]}).toBe(true);
      await page.getByLabel("Six-digit authenticator code").fill(authenticatorCode(fixtures[email]));
      await page.getByRole("button",{name:"Verify and continue",exact:true}).click();
    }
  }
  await expect(page.getByRole("button", { name: /Refresh/ })).toBeVisible();
}

async function accessible(page: Page) {
  const scan = await new AxeBuilder({ page }).analyze();
  expect(scan.violations.map(item => ({ id: item.id, impact: item.impact, nodeCount: item.nodes.length, nodes: item.nodes.slice(0, 5).map(node => node.target) }))).toEqual([]);
}

async function draftReport(page: Page, title: string, learnerName: string) {
  await page.getByRole("button", { name: "Create feedback", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Create learning feedback" });
  await dialog.getByRole("combobox", { name: "Learner", exact: true }).selectOption({ label: learnerName });
  await dialog.getByLabel("Feedback title").fill(title);
  for (const label of ["What we learned", "Strengths", "Focus next", "Practice plan", "Coach comment"]) await dialog.getByLabel(label, { exact: true }).fill(`${label}: observed legal play and careful explanations in class.`);
  const evidence = dialog.locator('input[name^="evidence-"]');
  for (let index = 0; index < await evidence.count(); index++) {
    await evidence.nth(index).fill("Observed in the learner’s submitted game analysis.");
    await dialog.locator(`select[name="rating-${index}"]`).selectOption("secure");
  }
  await dialog.getByRole("checkbox", { name: /Recommend progression/ }).check();
  await dialog.getByRole("button", { name: "Save feedback draft" }).click();
  await expect(dialog).not.toBeVisible();
}

test("real accounts complete academy teaching workflows and retain their sessions", async ({ browser, page }, testInfo) => {
  test.setTimeout(420000);
  const suffix = Date.now().toString(36);
  const coachName = `Journey Coach ${suffix}`;
  const coachEmail = `coach-${suffix}@example.test`;
  const guardianEmail = `guardian-${suffix}@example.test`;
  const learnerName = `Journey Learner ${suffix}`;
  const batchName = `Journey Batch ${suffix}`;
  let classStarts = 0;
  let classId = "";
  const homeworkTitle = `Explain central control ${suffix}`;
  const guardianContext = await browser.newContext({ baseURL: origin });
  const coachContext = await browser.newContext({ baseURL: origin });
  const studentContext = await browser.newContext({ baseURL: origin });
  const guardian = await guardianContext.newPage();
  const coach = await coachContext.newPage();
  const student = await studentContext.newPage();
  try {
    await test.step("admin signs in with email and enrolls MFA", async () => {
      await signIn(page, "browser-admin@example.test", true);
      await expect(page).toHaveURL(/\/admin$/);
      await request(page, "/academy/settings", { name: "Journey Academy", legalName: "Journey Academy Test", email: "academy@example.test" }, "PUT");
      for (const slug of ["privacy", "child-safety"]) await request(page, "/academy/policies", { slug, title: `Test ${slug} policy`, version: suffix, content: "Approved local test policy: guardians supervise access and control learner permissions." });
    });
    await test.step("admin provisions staff, learner, batch and enrollment through forms", async () => {
      await page.goto("/admin/settings");
      await page.getByText("Create a staff account", { exact: true }).click();
      await page.getByLabel("Staff name", { exact: true }).fill(coachName);
      await page.getByLabel("Staff email", { exact: true }).fill(coachEmail);
      await page.getByRole("combobox", { name: "Role", exact: true }).selectOption("coach");
      await page.getByRole("button", { name: "Create staff account" }).click();
      await expect(page.getByRole("status").filter({hasText:"Staff account created"})).toBeVisible();
      await page.goto("/admin/students");
      await page.getByRole("button", { name: "Add student", exact: true }).click();
      let dialog = page.getByRole("dialog");
      await dialog.getByLabel("Student name", { exact: true }).fill(learnerName);
      await dialog.getByLabel("Guardian name").fill("Journey Guardian");
      await dialog.getByLabel("Guardian email").fill(guardianEmail);
      await dialog.getByRole("button", { name: "Save student" }).click();
      await expect(dialog).not.toBeVisible();
      await expect(page.getByRole("heading", { name: learnerName, exact: true })).toBeVisible();
      await signIn(guardian, guardianEmail);
      await guardian.goto("/guardian/students");
      await guardian.getByRole("button", { name: "Review consent" }).click();
      await guardian.getByRole("checkbox", { name: /I am the guardian and accept/ }).check();
      await guardian.getByRole("button", { name: "Save consent", exact: true }).click();
      await expect(guardian.getByText("Guardian consent recorded", { exact: true })).toBeVisible();
      await page.reload();
      await page.getByRole("button", { name: "Create batch", exact: true }).click();
      dialog = page.getByRole("dialog");
      await dialog.getByLabel("Batch name").fill(batchName);
      await dialog.getByRole("combobox", { name: "Coach", exact: true }).selectOption({ label: coachName });
      await dialog.getByRole("button", { name: "Save batch" }).click();
      await expect(dialog).not.toBeVisible();
      await page.getByRole("button", { name: "Enroll learner", exact: true }).click();
      dialog = page.getByRole("dialog");
      await dialog.getByRole("combobox", { name: "Learner", exact: true }).selectOption({ label: learnerName });
      await dialog.getByRole("combobox", { name: "Batch", exact: true }).selectOption({ label: batchName });
      await dialog.getByRole("button", { name: "Save enrollment" }).click();
      await expect(dialog).not.toBeVisible();
      await page.goto("/admin/settings");
      await page.getByRole("combobox", { name: "Enrollment", exact: true }).selectOption({ label: `${learnerName} · ${batchName} · 0 available` });
      await page.getByLabel("Reference and reason").fill("Approved local test learning allowance");
      await page.getByRole("button", { name: "Grant credits", exact: true }).click();
      await expect(page.getByRole("status").first()).toContainText("Credits granted");
    });
    await test.step("assigned coach schedules a real class and uploads teaching material", async () => {
      await signIn(coach, coachEmail, true);
      await coach.goto("/coach/calendar");
      await coach.getByRole("button", { name: "Schedule class", exact: true }).click();
      const dialog = coach.getByRole("dialog");
      await dialog.getByLabel("Lesson title or topic").fill("Central control and candidate moves");
      classStarts = Math.ceil(Date.now() / 60000) * 60000 + 60000;
      await dialog.getByLabel("Local start", { exact: true }).fill(new Date(classStarts).toISOString().slice(0, 16));
      await dialog.getByLabel("Timezone", { exact: true }).fill("UTC");
      await dialog.getByLabel("Zoom or Meet link").fill("https://meet.google.com/abc-defg-hij");
      await dialog.getByRole("button", { name: "Preview schedule" }).click();
      await expect(dialog.getByRole("heading", { name: "1 class ready to schedule" })).toBeVisible();
      await dialog.getByRole("button", { name: "Confirm schedule" }).click();
      await expect(dialog).not.toBeVisible();
      await expect(coach.getByRole("heading", { name: "Central control and candidate moves" })).toBeVisible();
      const workspace = await request<AcademyWorkspaceData>(page, "/academy/workspace");
      classId = workspace.classes.find(item => item.batchId === workspace.batches.find(batch => batch.name === batchName)?.id)!.id;
      expect(workspace.enrolments.find(item => item.studentId === workspace.students.find(learner => learner.name === learnerName)?.id)).toMatchObject({ available: 7, reserved: 1 });
      await coach.goto("/coach/library");
      const upload = coach.locator(".workspace-panel").filter({ has: coach.getByRole("heading", { name: "Upload teaching material" }) });
      await upload.getByLabel("Resource title", { exact: true }).fill("Centre game study");
      await upload.getByLabel(/^Resource file/).setInputFiles({ name: "centre.pgn", mimeType: "application/x-chess-pgn", buffer: Buffer.from('[Event "Centre"]\n\n1. e4 e5 2. Nf3 Nc6 *') });
      await upload.getByRole("button", { name: "Upload file" }).click();
      await expect(upload.getByRole("status")).toContainText("uploaded and checked");
      const resource = coach.locator("article").filter({ has: coach.getByRole("heading", { name: "Centre game study", exact: true }) });
      await resource.getByRole("button", { name: "Publish", exact: true }).click();
      await expect(resource.getByText("personal · published")).toBeVisible();
    });
    await test.step("coach assigns versioned material and learner submits two reviewed attempts", async () => {
      await coach.goto("/coach/homework");
      await coach.getByRole("button", { name: "Create homework" }).click();
      const dialog = coach.getByRole("dialog");
      await dialog.getByLabel("Homework title").fill(homeworkTitle);
      await dialog.getByLabel("Instructions", { exact: true }).fill("Study the centre game and explain why central control helps development.");
      await dialog.getByText("Attach learning material", { exact: false }).click();
      await dialog.getByRole("checkbox", { name: /Centre game study/ }).check();
      await dialog.getByRole("button", { name: "Continue", exact: true }).click();
      await dialog.getByRole("checkbox", { name: learnerName, exact: true }).check();
      await dialog.getByRole("radio", { name: /Assign to learners/ }).check();
      await dialog.getByRole("button", { name: "Continue", exact: true }).click();
      await dialog.getByRole("button", { name: "Assign homework", exact: true }).click();
      await expect(dialog).not.toBeVisible();
      await page.goto("/admin/students");
      await page.locator("article").filter({has: page.getByRole("heading", {name: learnerName, exact: true})}).getByRole("button", { name: "Create student access code" }).click();
      const code = await page.locator(".academy-access-code").textContent();
      await student.goto("/student-access");
      await student.getByLabel("Student access code", { exact: true }).fill(code!.trim());
      await student.getByRole("button", { name: "Open student portal" }).click();
      await expect(student).toHaveURL(/\/student$/);
      await student.goto("/student/homework");
      await student.getByRole("button", { name: `View ${homeworkTitle}`, exact: true }).click();
      await expect(student.getByRole("heading", { name: homeworkTitle })).toBeVisible();
      await expect(student.getByText("Centre game study · version 2")).toBeVisible();
      await student.getByText("Centre game study · version 2", { exact: true }).click();
      const [material] = await Promise.all([student.waitForEvent("download"), student.getByRole("button", { name: "centre.pgn", exact: true }).click()]);
      expect(material.suggestedFilename()).toBe("centre.pgn");
      await student.getByRole("button", { name: "Start homework", exact: true }).click();
      await student.getByText("Attach practice evidence", { exact: true }).click();
      await student.getByLabel(/^Practice evidence/).setInputFiles({ name: "learner-analysis.pgn", mimeType: "application/x-chess-pgn", buffer: Buffer.from('[Event "Learner analysis"]\n\n1. e4 e5 2. Nf3 *') });
      await student.getByRole("button", { name: "Upload file", exact: true }).click();
      await expect(student.getByRole("status").filter({ hasText: "learner-analysis.pgn uploaded and checked" })).toBeVisible();
      await student.getByLabel("Your thinking").fill("Central pawns control squares and give the pieces room to develop.");
      await student.getByRole("button", { name: "Submit practice", exact: true }).click();
      await expect(student.getByRole("article").getByText("With your coach", { exact: true })).toBeVisible();
      await coach.reload();
      await coach.getByRole("button", { name: `View ${homeworkTitle} · ${learnerName}`, exact: true }).click();
      await coach.getByLabel("Coach feedback", { exact: true }).fill("Good start. Add the squares controlled by each central pawn.");
      await coach.getByRole("button", { name: "Return for another attempt" }).click();
      await expect(coach.getByRole("article").getByText("Needs another look", { exact: true })).toBeVisible();
      await student.reload();
      await student.getByRole("button", { name: `View ${homeworkTitle}`, exact: true }).click();
      await student.getByRole("button", { name: "Improve my answer", exact: true }).click();
      await student.getByLabel("Your thinking").fill("The e4 pawn controls d5 and f5; d4 controls c5 and e5. These squares support development.");
      await student.getByRole("button", { name: "Submit practice", exact: true }).click();
      await expect(student.getByRole("article").getByText("With your coach", { exact: true })).toBeVisible();
      await coach.reload();
      await coach.getByRole("button", { name: `View ${homeworkTitle} · ${learnerName}`, exact: true }).click();
      await coach.getByLabel("Coach feedback", { exact: true }).fill("Clear explanation with accurate square names. Keep checking the opponent’s threats.");
      await coach.getByRole("button", { name: "Share feedback", exact: true }).click();
      await expect(coach.getByText("Attempt history (2)")).toBeVisible();
      await guardian.goto("/guardian/homework");
      await guardian.getByRole("button", { name: /^Completed/ }).click();
      await guardian.getByRole("button", { name: `View ${homeworkTitle} · ${learnerName}`, exact: true }).click();
      await expect(guardian.getByText("Clear explanation with accurate square names.", { exact: false }).first()).toBeVisible();
      await expect(guardian.getByRole("button", { name: "Submit practice" })).toHaveCount(0);
    });
    await test.step("published reports retain corrections and support academic placement", async () => {
      await page.goto("/admin/progress");
      await page.getByRole("combobox", {name:"View learner", exact:true}).selectOption({label: learnerName});
      await draftReport(page, `Monthly central control report ${suffix}`, learnerName);
      await page.getByRole("button", { name: "Submit for review", exact: true }).click();
      await page.getByRole("button", { name: "Approve & publish", exact: true }).click();
      await guardian.goto("/guardian/progress");
      await guardian.getByRole("button", { name: "Mark as read", exact: true }).click();
      await expect(guardian.getByText("Read by guardian", { exact: false })).toBeVisible();
      await page.reload();
      await page.getByRole("combobox", {name:"View learner", exact:true}).selectOption({label: learnerName});
      await page.getByRole("button", { name: "Prepare a correction", exact: true }).click();
      const dialog = page.getByRole("dialog");
      await dialog.getByRole("textbox", { name: "Coach comment", exact: true }).fill("Corrected evidence: the learner independently explained both central pawn controls.");
      await dialog.getByRole("button", { name: "Save feedback draft" }).click();
      await expect(dialog).not.toBeVisible();
      await page.getByRole("button", { name: "Submit for review", exact: true }).click();
      await page.getByRole("button", { name: "Approve & publish", exact: true }).click();
      await expect(page.locator(".academy-assessment")).toHaveCount(2);
      await page.getByRole("button", { name: "Record placement", exact: true }).click();
      const placement = page.getByRole("dialog", { name: "Record curriculum placement" });
      await placement.getByRole("combobox", { name: "Curriculum level", exact: true }).selectOption("foundation-2");
      const evidence = placement.getByRole("combobox",{name: /^Published report evidence/});
      await evidence.selectOption({ index: 1 });
      await placement.getByLabel("Placement reason").fill("Published report demonstrates secure board foundations and readiness for tactical work.");
      await placement.getByRole("button", { name: "Record placement", exact: true }).click();
      await expect(placement).not.toBeVisible();
      await expect(page.getByText("Tactical foundations", { exact: true })).toBeVisible();
      const learning = await request<LearningWorkspace>(page, "/learning/workspace");
      const learnerId = learning.students.find(learner => learner.name === learnerName)!.id;
      expect(learning.assessments.filter(item => item.studentId === learnerId)).toHaveLength(2);
      expect(learning.placements.filter(item => item.studentId === learnerId)).toHaveLength(1);
      expect(learning.assessments.filter(item => item.studentId === learnerId && item.sourceVersionId)).toHaveLength(1);
      await page.getByRole("button", { name: "Record certificate", exact: true }).first().click();
      const certificates = page.locator(".workspace-panel").filter({ has: page.getByRole("heading", { name: "Level certificates", exact: true }) });
      await certificates.getByLabel(/^Certificate PDF/).setInputFiles({ name: "academy-certificate.pdf", mimeType: "application/pdf", buffer: Buffer.from("%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\n%%EOF") });
      await certificates.getByRole("button", { name: "Upload file", exact: true }).click();
      await expect(certificates.getByRole("button", { name: "Download certificate PDF" })).toBeVisible();
      await guardian.reload();
      const [certificate] = await Promise.all([guardian.waitForEvent("download"), guardian.getByRole("button", { name: "Download certificate PDF", exact: true }).click()]);
      expect(certificate.suggestedFilename()).toMatch(/certificate\.pdf$/);
    });
    await test.step("coach and learner synchronize a real classroom and record attendance", async () => {
      await coach.goto("/coach/calendar");
      const lesson = coach.getByRole("region", { name: "Class schedule", exact: true });
      await lesson.getByRole("link", { name: "Join classroom", exact: true }).click();
      await student.goto("/student/calendar");
      await student.getByRole("link", { name: "Join classroom", exact: true }).click();
      for (const classroom of [coach, student]) {
        await classroom.getByRole("button", { name: "Join classroom board", exact: true }).click();
        await expect(classroom.getByRole("heading", { name: "Your next good move.", exact: true })).toBeVisible();
      }
      await coach.getByText("Load a game or position", { exact: true }).click();
      await coach.getByLabel("PGN game", { exact: true }).fill("1. e4 e5 2. Nf3 *");
      await coach.getByRole("button", { name: "Import game to board", exact: true }).click();
      await expect(student.getByText("3 moves", { exact: true })).toBeVisible();
      for (const details of await coach.locator("details.classroom-tool-card").all()) {
        if (await details.getAttribute("open") === null) await details.locator("summary").click();
      }
      await coach.screenshot({path: testInfo.outputPath("classroom-desktop.png"), fullPage:true});
      await accessible(coach);
      await student.setViewportSize({width:390,height:844});
      await student.screenshot({path: testInfo.outputPath("classroom-mobile.png"), fullPage:true});
      await accessible(student);
      await student.setViewportSize({width:1280,height:720});
      await student.reload();
      await student.getByRole("button", { name: "Join classroom board", exact: true }).click();
      await expect(student.getByText("3 moves", { exact: true })).toBeVisible();
      await coach.getByRole("button", { name: "Save to game reviews", exact: true }).click();
      await expect(coach.getByRole("status").filter({ hasText: "Lesson game saved" })).toBeVisible();
      await expect.poll(() => Date.now() >= classStarts, { timeout: 125000, intervals: [1000] }).toBe(true);
      await coach.goto("/coach/calendar");
      await coach.getByRole("button", { name: "Attendance & notes", exact: true }).click();
      const attendance = coach.getByRole("dialog", { name: "Attendance & lesson notes", exact: true });
      await attendance.getByRole("combobox", { name: learnerName, exact: true }).selectOption("on_time");
      await attendance.getByLabel("Lesson notes", { exact: true }).fill("The learner reviewed central control and explained candidate moves on the shared board.");
      await attendance.getByRole("button", { name: "Complete class", exact: true }).click();
      await expect(attendance).not.toBeVisible();
      const workspace = await request<AcademyWorkspaceData>(page, "/academy/workspace");
      expect(workspace.classes.find(item => item.id === classId)?.status).toBe("completed");
      const enrollment = workspace.enrolments.find(item => item.batchId === workspace.batches.find(batch => batch.name === batchName)?.id);
      expect(enrollment).toMatchObject({ available: 7, reserved: 0 });
      const records = await request<LearningWorkspace>(student, "/learning/workspace");
      expect(records.games.some(game => game.source === "lesson" && game.pgn.includes("Nf3"))).toBe(true);
    });
    await test.step("board worker analyzes a position and navigation retains sign-in", async () => {
      await coach.goto("/coach/board");
      await coach.getByLabel("Enter a move", { exact: true }).fill("e4");
      await coach.getByRole("button", { name: "Play", exact: true }).click();
      await coach.getByRole("button", { name: "Engine", exact: true }).click();
      await expect(coach.getByText(/Line 1 · Depth/)).toBeVisible({ timeout: 30000 });
      await coach.getByRole("link", { name: "Back to academy", exact: false }).click();
      await expect(coach).toHaveURL(/\/coach$/);
      await coach.getByRole("navigation", { name: "Portal navigation" }).getByRole("link", { name: "Home", exact: true }).click();
      await coach.getByRole("link", { name: "KingsGambit2700 home", exact: true }).first().click();
      await coach.reload();
      await expect(coach.getByRole("button", { name: /Refresh/ })).toBeVisible();
      await coach.getByRole("link", { name: "Visit website", exact: true }).click();
      await coach.getByRole("link", { name: "My portal", exact: true }).first().click();
      await expect(coach).toHaveURL(/\/coach$/);
    });
    await test.step("guardian session restores on another browser context and signs out explicitly", async () => {
      const remembered = (await guardianContext.cookies()).filter(cookie => cookie.name === "kg_e2e_session_device");
      expect(remembered).toHaveLength(1);
      const restoredContext = await browser.newContext({ baseURL: origin });
      try {
        await restoredContext.addCookies(remembered);
        const restored = await restoredContext.newPage();
        await restored.goto("/guardian/homework");
        await expect(restored.getByRole("heading", { name: homeworkTitle })).toBeVisible();
        await accessible(restored);
        await restored.screenshot({path: testInfo.outputPath("workspace-desktop.png"), fullPage:true});
        await restored.setViewportSize({ width: 390, height: 844 });
        await accessible(restored);
        await restored.screenshot({path: testInfo.outputPath("workspace-mobile.png"), fullPage:true});
        expect(await restored.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)).toBe(true);
        await restored.getByRole("button", { name: "Open portal navigation" }).click();
        await restored.getByRole("button", { name: "Sign out", exact: true }).click();
        await expect(restored).toHaveURL(/\/sign-in(?:\?|$)/);
        await restored.reload();
        await expect(restored.getByRole("heading", { name: "Welcome back." })).toBeVisible();
      } finally { await restoredContext.close(); }
    });
    await student.goto("/admin");
    await expect(student).toHaveURL(/\/forbidden$/);
  } finally {
    await Promise.all([guardianContext.close(), coachContext.close(), studentContext.close()]);
  }
});
