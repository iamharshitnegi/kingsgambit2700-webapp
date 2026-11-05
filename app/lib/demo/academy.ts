import {
  type AcademyAvailability,
  type AcademyBatch,
  type AcademyClass,
  type AcademyEnrolment,
  type AcademyOperationsData,
  type AcademyStudent,
  type AcademyWorkspaceData,
  type CancellationPreview,
  type ClassInput,
} from "../academy-types";
import type { SessionUser } from "../api/types";
import { requestID } from "../request-id";
import { localTeachingInstant } from "../scheduling-time";
import { demoAccounts, type DemoWorkspace } from "./data";
import { DemoError, getDemoUser, writeDemo } from "./store";

type Profile = NonNullable<AcademyWorkspaceData["profile"]>;
export type DemoAcademyState = Omit<
  AcademyWorkspaceData,
  "capabilities" | "profile" | "workload" | "trialSlots" | "pagination"
> & {
  staff?: {
    id: string;
    displayName: string;
    email: string;
    status: string;
    roles: string[];
    createdAt: string;
    updatedAt: string;
  }[];
  profiles: Record<string, Profile>;
  guardianEmails: Record<string, string>;
  requests: Record<string, { fingerprint: string; result: unknown }>;
  series: Record<string, string>;
  resolved: Record<string, boolean>;
  operations: AcademyOperationsData;
};
const now = () => new Date().toISOString();
const json = (body: unknown) =>
  new Response(JSON.stringify(body), {
    headers: {
      "Content-Type": "application/json",
      "Cache-Control": "no-store",
    },
  });
const str = (input: Record<string, unknown>, key: string) =>
  String(input[key] || "").trim();
const required = (input: Record<string, unknown>, key: string) => {
  const value = str(input, key);
  if (!value) throw new DemoError(`Enter ${key}.`, 422);
  return value;
};
const roles = (user: SessionUser, ...allowed: string[]) =>
  user.roles.some((role) => allowed.includes(role));
function actor() {
  const user = getDemoUser();
  if (!user) throw new DemoError("Sign in to continue.", 401);
  return user;
}
function seed(state: DemoWorkspace): DemoAcademyState {
  if (state.academy) {
    state.academy.makeUps ||= [];
    state.academy.operations.pagination ||= {
      limit: 500,
      hasMore: false,
      nextLimit: 1000,
    };
    return state.academy;
  }
  const batchId = "demo-batch-foundations";
  state.academy = {
    makeUps: [],
    students: state.students.map((student) => ({
      id: student.id,
      userId: student.id,
      familyId: state.family.id,
      name: student.name,
      ageBand: student.ageBand,
      timeZone: state.family.timeZone,
      status: "active",
      consent: student.consent,
      country: "IN",
    })),
    coaches: [{ id: "demo-coach", name: "Coach Maya" }],
    batches: [
      {
        id: batchId,
        name: "Developing foundations",
        coachId: "demo-coach",
        format: "group",
        level: "beginner",
        timeZone: state.family.timeZone,
        capacity: 4,
        studentIds: state.students.map((student) => student.id),
      },
    ],
    enrolments: state.students.map((student) => ({
      id: `demo-enrolment-${student.id}`,
      studentId: student.id,
      batchId,
      accountId: `demo-credit-${student.id}`,
      status: "active",
      available: state.family.credits,
      reserved: 2,
      consumed: 1,
      startsAt: new Date(Date.now() - 30 * 86400000).toISOString(),
    })),
    classes: state.classes.map((lesson) => ({
      ...lesson,
      batchId,
      coachId: "demo-coach",
      format: "group",
      timeZone: state.family.timeZone,
      revision: 1,
      meetingUrl: "",
      attendance: Object.fromEntries(
        Object.entries(lesson.attendance).map(([id, value]) => [
          id,
          value === "present"
            ? "on_time"
            : value === "absent"
              ? "unexcused"
              : value,
        ]),
      ),
    })),
    policies: ["privacy", "child-safety"].map((slug) => ({
      id: `demo-policy-${slug}`,
      slug,
      title: slug === "child-safety" ? "Child safety" : "Privacy policy",
      version: "demo-1",
      content:
        "Sample academy policy for local exploration. A guardian confirms access and supervised online learning before lessons.",
    })),
    settings: {
      name: "KingsGambit2700",
      legalName: "Sample academy",
      email: "academy@example.test",
    },
    availability: [],
    profiles: {},
    guardianEmails: Object.fromEntries(
      state.students.map((student) => [student.id, "guardian@example.test"]),
    ),
    requests: {},
    series: {},
    resolved: {},
    operations: {
      pagination: { limit: 500, hasMore: false, nextLimit: 1000 },
      leads: [],
      trialSlots: [],
      trialBookings: [],
    },
  };
  return state.academy;
}
function save(state: DemoWorkspace) {
  const data = seed(state);
  data.makeUps ||= [];
  state.students = data.students.map((student) => ({
    ...state.students.find((old) => old.id === student.id),
    id: student.id,
    name: student.name,
    ageBand: student.ageBand,
    consent: student.consent,
    level:
      data.batches.find((batch) => batch.studentIds.includes(student.id))
        ?.level || "Awaiting placement",
  }));
  state.classes = data.classes.map((lesson) => ({
    ...lesson,
    status:
      lesson.status === "completed"
        ? "completed"
        : lesson.status.startsWith("cancelled")
          ? "cancelled"
          : "scheduled",
    attendance: Object.fromEntries(
      Object.entries(lesson.attendance).map(([id, value]) => [
        id,
        value === "on_time" || value === "late"
          ? "present"
          : value === "excused"
            ? "excused"
            : "absent",
      ]),
    ),
  }));
  state.family.credits = data.enrolments
    .filter(
      (item) =>
        data.students.find((student) => student.id === item.studentId)
          ?.familyId === state.family.id,
    )
    .reduce((sum, item) => sum + item.available, 0);
  writeDemo(state);
}
function allowedStudents(state: DemoWorkspace, user: SessionUser) {
  const data = seed(state);
  if (
    roles(
      user,
      "super_admin",
      "academic_manager",
      "operations_admin",
      "finance_admin",
    )
  )
    return data.students;
  const batches = new Set(
    data.batches
      .filter((batch) => batch.coachId === user.id)
      .map((batch) => batch.id),
  );
  return data.students.filter(
    (student) =>
      (roles(user, "student") && student.userId === user.id) ||
      (roles(user, "guardian") &&
        data.guardianEmails[student.id] === user.email) ||
      (roles(user, "coach") &&
        data.enrolments.some(
          (enrolment) =>
            enrolment.studentId === student.id &&
            enrolment.status === "active" &&
            batches.has(enrolment.batchId),
        )),
  );
}
export function demoAcademyWorkspace(
  state: DemoWorkspace,
): AcademyWorkspaceData {
  const user = actor();
  const data = seed(state);
  const students = allowedStudents(state, user);
  const ids = new Set(students.map((student) => student.id));
  const manageAcademy = roles(user, "super_admin", "operations_admin");
  const manageTeaching = roles(
    user,
    "coach",
    "academic_manager",
    "super_admin",
  );
  const batches = data.batches.filter(
    (batch) =>
      manageAcademy ||
      roles(user, "academic_manager", "finance_admin") ||
      (roles(user, "coach") && batch.coachId === user.id) ||
      batch.studentIds.some((id) => ids.has(id)),
  ).map((batch) => ({
    ...batch,
    studentIds: batch.studentIds.filter((id) => ids.has(id)),
  }));
  const classes = data.classes.filter(
    (lesson) =>
      manageAcademy ||
      roles(user, "academic_manager") ||
      (roles(user, "coach") && lesson.coachId === user.id) ||
      (roles(user, "guardian", "student") &&
        lesson.studentIds.some((id) => ids.has(id))),
  ).map((lesson) => ({
    ...lesson,
    studentIds: lesson.studentIds.filter((id) => ids.has(id)),
    attendance: Object.fromEntries(
      Object.entries(lesson.attendance).filter(([id]) => ids.has(id)),
    ),
  }));
  const coachIds = new Set(
    data.batches
      .filter((batch) => data.enrolments.some((enrolment) =>
        enrolment.batchId === batch.id &&
        enrolment.status === "active" && ids.has(enrolment.studentId),
      ))
      .map((batch) => batch.coachId),
  );
  const trialSlots = data.operations.trialSlots.filter(
    (slot) =>
      manageAcademy ||
      roles(user, "academic_manager") ||
      slot.coachId === user.id,
  );
  const monthStart = new Date();
  monthStart.setUTCDate(1);
  const months = [0, 1].map((offset) => {
    const date = new Date(monthStart);
    date.setUTCMonth(date.getUTCMonth() - offset);
    const month = date.toISOString().slice(0, 7);
    const lessons = classes.filter(
      (item) => item.status === "completed" && item.startsAt.startsWith(month),
    );
    const trials = trialSlots.filter(
      (slot) =>
        slot.startsAt.startsWith(month) &&
        data.operations.trialBookings.some(
          (booking) =>
            booking.slotId === slot.id && booking.status === "attended",
        ),
    );
    const minutes = (items: { startsAt: string; endsAt: string }[]) =>
      items.reduce(
        (total, item) =>
          total + (Date.parse(item.endsAt) - Date.parse(item.startsAt)) / 60000,
        0,
      );
    return {
      month,
      completedClasses: lessons.length,
      completedClassMinutes: minutes(lessons),
      completedTrials: trials.length,
      completedTrialMinutes: minutes(trials),
    };
  });
  return structuredClone({
    pagination: { limit: 500, hasMore: false, nextLimit: 1000 },
    makeUps: (data.makeUps || []).filter((item) => ids.has(item.studentId)),
    trialSlots,
    capabilities: {
      manageAcademy,
      manageTeaching,
      manageScheduling: manageAcademy || roles(user, "coach"),
      manageStaff: roles(user, "super_admin"),
      managePolicies: roles(user, "super_admin"),
    },
    students,
    batches,
    classes,
    coaches: data.coaches.filter((coach) =>
      data.staff?.find((person) => person.id === coach.id)?.status !== "suspended" &&
      (manageAcademy || coach.id === user.id || coachIds.has(coach.id)),
    ),
    enrolments: data.enrolments.filter((item) => ids.has(item.studentId)),
    policies: data.policies,
    settings: data.settings,
    availability:
      data.availability?.filter(
        (item) => manageAcademy || item.coachId === user.id,
      ) || [],
    profile: data.profiles[user.id] || {
      displayName: user.displayName || "",
      timeZone: "Asia/Kolkata",
      savedViews: [],
    },
    workload: {
      months,
      scheduledClasses: classes.filter((item) => item.status === "scheduled")
        .length,
      pendingCompletion: classes.filter(
        (item) =>
          item.status === "scheduled" && Date.parse(item.endsAt) < Date.now(),
      ).length,
      completedClasses: classes.filter((item) => item.status === "completed")
        .length,
      completedMinutes: classes
        .filter((item) => item.status === "completed")
        .reduce(
          (sum, item) =>
            sum + (Date.parse(item.endsAt) - Date.parse(item.startsAt)) / 60000,
          0,
        ),
    },
  });
}
function scheduleAllowed(user: SessionUser, coachId: string) {
  if (
    !roles(user, "operations_admin", "super_admin") &&
    !(roles(user, "coach") && coachId === user.id)
  )
    throw new DemoError(
      "Only the assigned coach or operations can schedule this class.",
      403,
    );
}
function duplicate(
  data: DemoAcademyState,
  key: string,
  fingerprint: string,
): unknown | undefined {
  const existing = data.requests[key];
  if (!existing) return;
  if (existing.fingerprint !== fingerprint)
    throw new DemoError(
      "This request identifier was already used for different details.",
      409,
    );
  return existing.result;
}
function active(lesson: AcademyClass) {
  return (
    !lesson.status.startsWith("cancelled") && lesson.status !== "rescheduled"
  );
}
function schedule(
  state: DemoWorkspace,
  input: ClassInput,
  ignore = new Set<string>(),
): AcademyClass[] {
  const data = seed(state);
  const user = actor();
  const batch = data.batches.find((item) => item.id === input.batchId);
  if (!batch) throw new DemoError("Choose a batch.", 422);
  scheduleAllowed(user, batch.coachId);
  if (
    !input.title?.trim() ||
    input.durationMinutes < 15 ||
    input.durationMinutes > 240 ||
    !Number.isInteger(input.occurrences) ||
    input.occurrences < 1 ||
    input.occurrences > 52
  )
    throw new DemoError(
      "Check the topic, duration, and number of weekly classes.",
      422,
    );
  if (input.meetingUrl) {
    const link = new URL(input.meetingUrl);
    if (
      link.protocol !== "https:" ||
      !(
        link.hostname === "meet.google.com" ||
        link.hostname === "zoom.us" ||
        link.hostname.endsWith(".zoom.us")
      )
    )
      throw new DemoError("Enter an HTTPS Zoom or Google Meet link.", 422);
  }
  const roster = data.enrolments.filter(
    (item) => item.batchId === batch.id && item.status === "active",
  );
  if (!roster.length || roster.length > batch.capacity)
    throw new DemoError(
      "Check enrollment and batch capacity before scheduling.",
      409,
    );
  if (
    roster.some(
      (item) =>
        !data.students.find((student) => student.id === item.studentId)
          ?.consent,
    )
  )
    throw new DemoError("Each learner needs guardian consent.", 409);
  if (!ignore.size && roster.some((item) => item.available < input.occurrences))
    throw new DemoError(
      "Each learner needs enough class credits for the complete schedule.",
      409,
    );
  const lessons: AcademyClass[] = [];
  for (let index = 0; index < input.occurrences; index++) {
    const date = new Date(`${input.localStart}:00Z`);
    date.setUTCDate(date.getUTCDate() + index * 7);
    const local = date.toISOString().slice(0, 16);
    if (
      input.occurrences > 1 &&
      (!input.recurringUntil || local.slice(0, 10) > input.recurringUntil)
    )
      throw new DemoError(
        "Choose an explicit final date for the weekly schedule.",
        422,
      );
    let startsAt: string;
    try {
      startsAt = localTeachingInstant(local, input.timeZone);
    } catch (cause) {
      throw new DemoError(
        cause instanceof Error ? cause.message : "Invalid timezone.",
        422,
      );
    }
    const endsAt = new Date(
      Date.parse(startsAt) + input.durationMinutes * 60000,
    ).toISOString();
    const studentIds = roster.map((item) => item.studentId);
    if (
      roster.some(
        (item) =>
          Date.parse(item.startsAt) > Date.parse(startsAt) ||
          (item.endsAt && Date.parse(item.endsAt) < Date.parse(endsAt)),
      )
    )
      throw new DemoError(
        "The class must fall within each learner's enrollment.",
        409,
      );
    if (
      data.classes.some(
        (item) =>
          !ignore.has(item.id) &&
          active(item) &&
          (item.coachId === batch.coachId ||
            item.studentIds.some((id) => studentIds.includes(id))) &&
          item.startsAt < endsAt &&
          item.endsAt > startsAt,
      )
    )
      throw new DemoError(
        "This time overlaps an existing coach or learner class.",
        409,
      );
    const availability =
      data.availability?.filter((item) => item.coachId === batch.coachId) || [];
    if (
      availability.some(
        (item) =>
          item.status === "blocked" &&
          item.startsAt < endsAt &&
          item.endsAt > startsAt,
      ) ||
      (availability.some((item) => item.status === "available") &&
        !availability.some(
          (item) =>
            item.status === "available" &&
            item.startsAt <= startsAt &&
            item.endsAt >= endsAt,
        ))
    )
      throw new DemoError(
        "This time is outside the coach's availability.",
        409,
      );
    lessons.push({
      id: requestID(),
      batchId: batch.id,
      title: input.title,
      coachId: batch.coachId,
      coach:
        data.coaches.find((item) => item.id === batch.coachId)?.name || "Coach",
      format: batch.format,
      studentIds,
      startsAt,
      endsAt,
      timeZone: input.timeZone,
      status: "scheduled",
      revision: 1,
      notes: "",
      meetingUrl: input.meetingUrl || "",
      attendance: {},
    });
  }
  return lessons;
}

export function demoAcademyRequest(
  path: string,
  method: string,
  body: Record<string, unknown>,
  state: DemoWorkspace,
  request: Request,
): Response | null {
  if (
    !path.startsWith("/academy/") &&
    !/^\/entitlements\/accounts\/[^/]+\/grants$/.test(path)
  )
    return null;
  const user = actor();
  const data = seed(state);
  const view = demoAcademyWorkspace(state);
  const admin = view.capabilities.manageAcademy;
  const requireAdmin = () => {
    if (!admin)
      throw new DemoError("Operations or super-admin access is required.", 403);
  };
  const foundStudent = (id: string) => {
    const student = view.students.find((item) => item.id === id);
    if (!student) throw new DemoError("Learner not found.", 404);
    return student;
  };
  const commit = (result: unknown) => {
    state.audit.unshift({
      id: requestID(),
      actor: user.displayName || user.id,
      action: `${method} ${path}`,
      at: now(),
    });
    save(state);
    return json(result);
  };
  if (path === "/academy/workspace" && method === "GET") return json(view);
  if (path === "/academy/profile" || path === "/academy/preferences") {
    const current = data.profiles[user.id] || view.profile!;
    const timeZone = str(body, "timeZone") || current.timeZone;
    try {
      new Intl.DateTimeFormat("en", { timeZone });
    } catch {
      throw new DemoError("Choose a valid timezone.", 422);
    }
    data.profiles[user.id] = {
      displayName: path.endsWith("profile")
        ? required(body, "displayName")
        : current.displayName,
      timeZone,
      savedViews: Array.isArray(body.savedViews)
        ? (body.savedViews as Profile["savedViews"])
        : current.savedViews,
    };
    return commit(data.profiles[user.id]);
  }
  if (path === "/academy/students" && method === "POST") {
    if (!admin && !roles(user, "guardian"))
      throw new DemoError("Family or operations access is required.", 403);
    const id = requestID();
    const email =
      roles(user, "guardian") && !admin
        ? user.email!
        : required(body, "guardianEmail");
    const student: AcademyStudent = {
      id,
      userId: id,
      familyId:
        email === "guardian@example.test" ? state.family.id : requestID(),
      name: required(body, "name"),
      ageBand: required(body, "ageBand"),
      timeZone: str(body, "timeZone") || "Asia/Kolkata",
      country: str(body, "country") || "IN",
      status: "prospect",
      consent: false,
    };
    data.students.push(student);
    data.guardianEmails[id] = email;
    return commit(student);
  }
  const studentPath = path.match(
    /^\/academy\/students\/([^/]+)(?:\/(consent|access-code))?$/,
  );
  if (studentPath) {
    const student = data.students.find(
      (item) => item.id === foundStudent(studentPath[1]).id,
    )!;
    if (
      !admin &&
      !(
        roles(user, "guardian") &&
        data.guardianEmails[student.id] === user.email
      )
    )
      throw new DemoError(
        "Only family or operations may update this learner.",
        403,
      );
    if (studentPath[2] === "consent") {
      if (
        !roles(user, "guardian") ||
        data.guardianEmails[student.id] !== user.email
      )
        throw new DemoError("The guardian must record consent.", 403);
      if (
        !Array.isArray(body.policyVersionIds) ||
        data.policies
          .filter((item) => ["privacy", "child-safety"].includes(item.slug))
          .some(
            (item) => !(body.policyVersionIds as string[]).includes(item.id),
          )
      )
        throw new DemoError(
          "Accept the current privacy and child-safety policies.",
          422,
        );
      student.consent = true;
      return commit({ accepted: true });
    }
    if (studentPath[2] === "access-code") {
      if (!student.consent)
        throw new DemoError("Record guardian consent first.", 409);
      return json({
        code: "DEMO2700",
        expiresAt: new Date(Date.now() + 900000).toISOString(),
      });
    }
    if (method === "PATCH") {
      Object.assign(student, {
        name: required(body, "name"),
        ageBand: required(body, "ageBand"),
        timeZone: required(body, "timeZone"),
        country: str(body, "country"),
        status: required(body, "status"),
      });
      return commit(student);
    }
  }
  if (path === "/academy/staff" || /^\/academy\/staff\/[^/]+$/.test(path)) {
    if (!view.capabilities.manageStaff)
      throw new DemoError("Super-admin access is required.", 403);
    const seedStaff = !data.staff;
    data.staff ||= demoAccounts
      .filter((account) => !["student", "guardian"].includes(account.role))
      .map((account) => ({
        id: `demo-${account.role}`,
        displayName: account.name,
        email: account.email,
        roles: [account.role],
        status: "active",
        createdAt: now(),
        updatedAt: now(),
      }));
    if (seedStaff) save(state);
    if (method === "GET") return json({ staff: data.staff });
    if (method === "PATCH") {
      const person = data.staff.find((item) => item.id === path.split("/")[3]);
      if (!person) throw new DemoError("Staff member not found.", 404);
      if (person.updatedAt !== body.expectedUpdatedAt)
        throw new DemoError(
          "This staff member changed. Refresh before editing.",
          409,
        );
      if (!["active", "suspended"].includes(String(body.status)))
        throw new DemoError("Select an account status.", 422);
      if (person.id === user.id && body.status === "suspended")
        throw new DemoError("You cannot suspend your own account.", 409);
      person.displayName = required(body, "displayName");
      person.status = String(body.status);
      person.updatedAt = now();
      const coach = data.coaches.find((item) => item.id === person.id);
      if (coach) coach.name = person.displayName;
      return commit(person);
    }
    if (
      !demoAccounts.some(
        (account) =>
          account.role === body.role &&
          !["student", "guardian"].includes(account.role),
      )
    )
      throw new DemoError("Choose a staff role.", 422);
    const coach = { id: requestID(), name: required(body, "name") };
    const email = required(body, "email").toLowerCase();
    if (data.staff.some((person) => person.email === email))
      throw new DemoError("A staff member already uses this email.", 409);
    data.staff.push({
      id: coach.id,
      displayName: coach.name,
      email,
      roles: [String(body.role)],
      status: "active",
      createdAt: now(),
      updatedAt: now(),
    });
    if (body.role === "coach") data.coaches.push(coach);
    return commit(coach);
  }
  if (/^\/academy\/batches(?:\/[^/]+)?$/.test(path)) {
    requireAdmin();
    const existing = data.batches.find(
      (item) => item.id === path.split("/")[3],
    );
    if (method === "PATCH" && !existing)
      throw new DemoError("Batch not found.", 404);
    const capacity = Number(body.capacity);
    if (
      !Number.isInteger(capacity) ||
      capacity < 1 ||
      capacity > 6 ||
      (body.format === "one_to_one" && capacity !== 1)
    )
      throw new DemoError("Choose a valid batch capacity.", 422);
    const batch: AcademyBatch = {
      id: existing?.id || requestID(),
      name: required(body, "name"),
      coachId: required(body, "coachId"),
      format: required(body, "format"),
      level: required(body, "level"),
      timeZone: required(body, "timeZone"),
      capacity,
      studentIds: existing?.studentIds || [],
    };
    if (batch.studentIds.length > capacity)
      throw new DemoError("Capacity is below current enrollment.", 409);
    if (!data.coaches.some((item) => item.id === batch.coachId))
      throw new DemoError("Choose an academy coach.", 422);
    if (existing) Object.assign(existing, batch);
    else data.batches.push(batch);
    return commit(batch);
  }
  if (path === "/academy/enrolments") {
    requireAdmin();
    const student = foundStudent(required(body, "studentId"));
    const batch = data.batches.find((item) => item.id === body.batchId);
    if (!batch) throw new DemoError("Choose a batch.", 422);
    if (!student.consent)
      throw new DemoError("Record guardian consent before enrollment.", 409);
    if (
      batch.studentIds.length >= batch.capacity ||
      batch.studentIds.includes(student.id)
    )
      throw new DemoError(
        "The batch is full or learner is already enrolled.",
        409,
      );
    const enrolment: AcademyEnrolment = {
      id: requestID(),
      studentId: student.id,
      batchId: batch.id,
      accountId: requestID(),
      status: "active",
      available: 0,
      reserved: 0,
      consumed: 0,
      startsAt: required(body, "startsAt"),
      ...(body.endsAt ? { endsAt: String(body.endsAt) } : {}),
    };
    data.enrolments.push(enrolment);
    batch.studentIds.push(student.id);
    data.students.find((item) => item.id === student.id)!.status = "active";
    return commit(enrolment);
  }
  if (path.startsWith("/academy/enrolments/")) {
    requireAdmin();
    const item = data.enrolments.find((item) => item.id === path.split("/")[3]);
    if (!item) throw new DemoError("Enrollment not found.", 404);
    item.status = required(body, "status");
    item.endsAt = str(body, "endsAt") || undefined;
    const batch = data.batches.find((batch) => batch.id === item.batchId);
    if (batch)
      batch.studentIds = data.enrolments
        .filter(
          (entry) => entry.batchId === batch.id && entry.status === "active",
        )
        .map((entry) => entry.studentId);
    return commit(item);
  }
  if (path.startsWith("/entitlements/accounts/")) {
    if (!roles(user, "finance_admin", "super_admin"))
      throw new DemoError("Finance access is required.", 403);
    const item = data.enrolments.find(
      (item) => item.accountId === path.split("/")[3],
    );
    if (!item) throw new DemoError("Credit account not found.", 404);
    const key =
      request.headers.get("Idempotency-Key") || required(body, "referenceId");
    const fingerprint = JSON.stringify(body);
    const previous = duplicate(data, key, fingerprint);
    if (previous) return json(previous);
    const units = Number(body.units);
    if (!Number.isInteger(units) || units < 1)
      throw new DemoError("Enter a positive number of credits.", 422);
    required(body, "reason");
    item.available += units;
    data.requests[key] = { fingerprint, result: item };
    return commit(item);
  }
  if (path === "/academy/classes" || path === "/academy/classes/preview") {
    const input = body as unknown as ClassInput;
    const fingerprint = JSON.stringify(body);
    if (path.endsWith("preview"))
      return json({ classes: schedule(state, input) });
    const previous = duplicate(
      data,
      required(body, "idempotencyKey"),
      fingerprint,
    );
    if (previous) return json(previous);
    const classes = schedule(state, input);
    for (const lesson of classes) {
      data.classes.push(lesson);
      data.series[lesson.id] = input.idempotencyKey;
      for (const id of lesson.studentIds) {
        const account = data.enrolments.find(
          (item) =>
            item.studentId === id &&
            item.batchId === lesson.batchId &&
            item.status === "active",
        )!;
        account.available--;
        account.reserved++;
      }
    }
    const result = { classes };
    data.requests[input.idempotencyKey] = { fingerprint, result };
    return commit(result);
  }
  const classPath = path.match(
    /^\/academy\/classes\/([^/]+)(?:\/(complete|cancel|cancel-preview))?$/,
  );
  if (classPath) {
    const lesson = data.classes.find((item) => item.id === classPath[1]);
    if (!lesson || !view.classes.some((item) => item.id === lesson.id))
      throw new DemoError("Class not found.", 404);
    const fingerprint = JSON.stringify(body);
    const key = str(body, "idempotencyKey");
    const previous = key ? duplicate(data, key, fingerprint) : undefined;
    if (previous) return json(previous);
    if (Number(body.revision) !== lesson.revision)
      throw new DemoError("This class changed. Refresh before saving.", 409);
    if (method === "PATCH") {
      scheduleAllowed(user, lesson.coachId);
      const affected =
        body.scope === "future" && data.series[lesson.id]
          ? data.classes.filter(
              (item) =>
                data.series[item.id] === data.series[lesson.id] &&
                item.startsAt >= lesson.startsAt &&
                item.status === "scheduled",
            )
          : [lesson];
      const draft = affected.map((item, index) => {
        const date = new Date(`${body.localStart}:00Z`);
        date.setUTCDate(date.getUTCDate() + index * 7);
        return schedule(
          state,
          {
            ...body,
            localStart: date.toISOString().slice(0, 16),
            occurrences: 1,
          } as unknown as ClassInput,
          new Set(affected.map((item) => item.id)),
        )[0];
      });
      affected.forEach((item, index) =>
        Object.assign(item, draft[index], {
          id: item.id,
          revision: item.revision + 1,
        }),
      );
      return commit(lesson);
    }
    if (classPath[2] === "complete") {
      if (
        !roles(user, "academic_manager", "super_admin") &&
        !(roles(user, "coach") && lesson.coachId === user.id)
      )
        throw new DemoError(
          "Only the assigned teaching team may complete a class.",
          403,
        );
      if (
        lesson.status !== "scheduled" ||
        Date.parse(lesson.startsAt) > Date.now()
      )
        throw new DemoError("This class is not ready for completion.", 409);
      const attendance = body.attendance as Record<string, string>;
      if (
        !attendance ||
        lesson.studentIds.some(
          (id) =>
            ![
              "on_time",
              "late",
              "excused",
              "unexcused",
              "technical_failure",
            ].includes(attendance[id]),
        )
      )
        throw new DemoError("Resolve attendance for every learner.", 422);
      if (required(body, "notes").length < 10)
        throw new DemoError("Add a useful lesson note.", 422);
      lesson.attendance = attendance;
      lesson.notes = String(body.notes);
      lesson.status = "completed";
      for (const id of lesson.studentIds) {
        const account = data.enrolments.find(
          (item) => item.studentId === id && item.batchId === lesson.batchId,
        );
        if (
          account &&
          account.reserved &&
          !data.resolved[`${lesson.id}:${id}`]
        ) {
          account.reserved--;
          account.consumed++;
          data.resolved[`${lesson.id}:${id}`] = true;
        }
      }
      lesson.revision++;
      data.requests[key] = { fingerprint, result: lesson };
      return commit(lesson);
    }
    const studentId = str(body, "studentId");
    if (studentId && !lesson.studentIds.includes(studentId))
      throw new DemoError("Learner is not in this class.", 422);
    if (
      !admin &&
      !(roles(user, "coach") && lesson.coachId === user.id) &&
      !(
        studentId &&
        roles(user, "guardian") &&
        data.guardianEmails[studentId] === user.email
      )
    )
      throw new DemoError("This cancellation is outside your access.", 403);
    if (!studentId && !admin && !roles(user, "coach"))
      throw new DemoError("Select your learner.", 403);
    const early =
      lesson.format === "one_to_one" &&
      Date.parse(lesson.startsAt) - Date.now() >= 24 * 3600000;
    const outcomes = (studentId ? [studentId] : lesson.studentIds).map(
      (id) => ({
        studentId: id,
        action: !studentId || early ? "restore" : "consume",
        makeUp: !studentId,
        reason: !studentId
          ? "academy_cancellation"
          : lesson.format === "group"
            ? "group_family_absence"
            : early
              ? "one_to_one_before_cutoff"
              : "one_to_one_late_cancellation",
      }),
    );
    const result: CancellationPreview = {
      classId: lesson.id,
      revision: lesson.revision,
      outcomes,
    };
    if (classPath[2] === "cancel-preview") return json(result);
    required(body, "reason");
    if (lesson.status !== "scheduled")
      throw new DemoError("This class can no longer be cancelled.", 409);
    for (const outcome of outcomes) {
      const account = data.enrolments.find(
        (item) =>
          item.studentId === outcome.studentId &&
          item.batchId === lesson.batchId,
      );
      if (data.resolved[`${lesson.id}:${outcome.studentId}`])
        throw new DemoError(
          "This learner already has a resolved cancellation.",
          409,
        );
      if (account && account.reserved) {
        account.reserved--;
        if (outcome.action === "restore") account.available++;
        else account.consumed++;
        data.resolved[`${lesson.id}:${outcome.studentId}`] = true;
      }
      if (outcome.makeUp)
        data.makeUps.push({
          id: requestID(),
          classId: lesson.id,
          studentId: outcome.studentId,
          reason: outcome.reason,
          status: "open",
          createdAt: now(),
        });
    }
    if (studentId && lesson.format === "group")
      lesson.attendance[studentId] = "excused";
    else lesson.status = studentId ? "cancelled_family" : "cancelled_academy";
    lesson.revision++;
    data.requests[key] = { fingerprint, result };
    return commit(result);
  }
  const makeUpPath = path.match(/^\/academy\/make-ups\/([^/]+)\/fulfill$/);
  if (makeUpPath && method === "POST") {
    const item = data.makeUps.find(
      (candidate) =>
        candidate.id === makeUpPath[1] &&
        view.students.some((student) => student.id === candidate.studentId),
    );
    if (!item) throw new DemoError("Make-up not found.", 404);
    const replacement = data.classes.find(
      (lesson) =>
        lesson.id === body.replacementClassId &&
        lesson.studentIds.includes(item.studentId),
    );
    if (!replacement)
      throw new DemoError("Select a replacement class for this learner.", 422);
    scheduleAllowed(user, replacement.coachId);
    if (
      item.status === "fulfilled" &&
      item.replacementClassId === replacement.id
    )
      return json(item);
    if (
      item.status !== "open" ||
      replacement.status !== "scheduled" ||
      Date.parse(replacement.startsAt) <= Date.now() ||
      data.resolved[`${replacement.id}:${item.studentId}`] ||
      data.makeUps.some(
        (other) =>
          other.studentId === item.studentId &&
          other.replacementClassId === replacement.id,
      )
    )
      throw new DemoError(
        "Select an upcoming reserved class that is not already a make-up.",
        409,
      );
    item.status = "fulfilled";
    item.replacementClassId = replacement.id;
    return commit(item);
  }
  if (path === "/academy/availability") {
    if (method === "GET") return json(view.availability);
    const coachId = required(body, "coachId");
    scheduleAllowed(user, coachId);
    const item: AcademyAvailability = {
      id: requestID(),
      coachId,
      startsAt: required(body, "startsAt"),
      endsAt: required(body, "endsAt"),
      timeZone: required(body, "timeZone"),
      status: body.status === "blocked" ? "blocked" : "available",
    };
    if (item.startsAt >= item.endsAt)
      throw new DemoError("End must follow start.", 422);
    data.availability ??= [];
    data.availability.push(item);
    return commit(item);
  }
  if (path.startsWith("/academy/availability/") && method === "DELETE") {
    const item = data.availability?.find(
      (item) => item.id === path.split("/")[3],
    );
    if (!item) throw new DemoError("Availability not found.", 404);
    scheduleAllowed(user, item.coachId);
    data.availability = data.availability?.filter(
      (entry) => entry.id !== item.id,
    );
    save(state);
    return new Response(null, { status: 204 });
  }
  if (path === "/academy/policies" || path === "/academy/settings") {
    if (!view.capabilities.managePolicies)
      throw new DemoError("Super-admin access is required.", 403);
    if (path.endsWith("settings")) {
      data.settings = {
        name: required(body, "name"),
        legalName: required(body, "legalName"),
        email: required(body, "email"),
      };
      return commit(data.settings);
    }
    const policy = {
      id: requestID(),
      slug: required(body, "slug"),
      title: required(body, "title"),
      version: required(body, "version"),
      content: required(body, "content"),
    };
    data.policies = data.policies.filter((item) => item.slug !== policy.slug);
    data.policies.push(policy);
    return commit(policy);
  }
  if (path === "/academy/operations") {
    requireAdmin();
    return json({
      ...data.operations,
      leads: state.leads.map((item) => ({
        ...item,
        notes: "",
        phone: "",
        ageBand: item.studentAgeBand || "8_12",
        email: item.email || "",
        ...data.operations.leads.find((lead) => lead.id === item.id),
      })),
      trialBookings: state.bookings.map((item) => ({
        ...item,
        studentName: "",
        notes: "",
        ...data.operations.trialBookings.find(
          (booking) => booking.id === item.id,
        ),
      })),
    });
  }
  if (path.startsWith("/academy/leads/")) {
    requireAdmin();
    const id = path.split("/")[3];
    const source = state.leads.find((item) => item.id === id);
    if (!source) throw new DemoError("Enquiry not found.", 404);
    let item = data.operations.leads.find((item) => item.id === id);
    if (!item) {
      item = {
        ...source,
        email: source.email || "",
        phone: source.phone || "",
        ageBand: source.studentAgeBand || "",
        notes: "",
      };
      data.operations.leads.push(item);
    }
    if (path.endsWith("/convert")) {
      const student = foundStudent(required(body, "studentId"));
      if (
        data.guardianEmails[student.id] !== item.email ||
        !data.enrolments.some(
          (entry) =>
            entry.studentId === student.id && entry.status === "active",
        )
      )
        throw new DemoError(
          "Choose an enrolled learner whose guardian matches this enquiry.",
          409,
        );
      item.studentId = student.id;
      item.status = "won";
    } else {
      item.notes = str(body, "notes");
      item.status = required(body, "status");
    }
    source.status = item.status;
    return commit(item);
  }
  if (path === "/academy/trial-slots") {
    requireAdmin();
    const slot = {
      id: requestID(),
      coachId: required(body, "coachId"),
      startsAt: required(body, "startsAt"),
      endsAt: required(body, "endsAt"),
      timeZone: required(body, "timeZone"),
      format: required(body, "format"),
      capacity: Number(body.capacity),
      ageBands: body.ageBands as string[],
      status: "published",
      availableSeats: Number(body.capacity),
    };
    if (slot.startsAt >= slot.endsAt || slot.capacity < 1 || slot.capacity > 6)
      throw new DemoError("Check trial times and capacity.", 422);
    data.operations.trialSlots.push(slot);
    return commit(slot);
  }
  if (path.startsWith("/academy/trial-bookings/")) {
    requireAdmin();
    const booking = state.bookings.find(
      (item) => item.id === path.split("/")[3],
    );
    if (!booking) throw new DemoError("Trial booking not found.", 404);
    const item = { ...booking, studentName: "", notes: str(body, "notes") };
    if (path.endsWith("reschedule")) {
      const slot = data.operations.trialSlots.find(
        (slot) => slot.id === body.slotId,
      );
      if (!slot || slot.availableSeats < 1)
        throw new DemoError("Choose an available trial slot.", 409);
      Object.assign(booking, {
        slotId: slot.id,
        startsAt: slot.startsAt,
        endsAt: slot.endsAt,
      });
      slot.availableSeats--;
    } else booking.status = required(body, "status");
    Object.assign(item, booking);
    data.operations.trialBookings = data.operations.trialBookings.filter(
      (old) => old.id !== item.id,
    );
    data.operations.trialBookings.push(item);
    return commit(item);
  }
  throw new DemoError("This academy operation is unavailable.", 404);
}
