"use client";

import { useEffect, useRef, useState } from "react";
import CalendarView from "./CalendarView";
import { Icon } from "@/app/components/Icon";
import { apiFetch } from "@/app/lib/api/client";
import type {
  AcademyClass,
  AcademyMutation,
  AcademyWorkspaceData,
  CancellationPreview,
  ClassInput,
} from "@/app/lib/academy-types";
import {
  Badge,
  Empty,
  Field,
  Modal,
  formValues,
  label,
  localInput,
  time,
  value,
} from "./WorkspaceUI";

function ClassEditor({
  data,
  lesson,
  close,
  mutate,
  saved,
}: {
  data: AcademyWorkspaceData;
  saved: (input: ClassInput, id?: string) => void;
  lesson?: AcademyClass;
  close: () => void;
  mutate: AcademyMutation;
}) {
  const [batchId, setBatch] = useState(
    lesson?.batchId || data.batches[0]?.id || "",
  );
  const batch = data.batches.find((item) => item.id === batchId);
  const [preview, setPreview] = useState<{
    input: ClassInput;
    classes: AcademyClass[];
  } | null>(null);
  const reviewHeading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    if (preview) reviewHeading.current?.focus();
  }, [preview]);
  const [weekly, setWeekly] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [key] = useState(() => crypto.randomUUID());
  return (
    <Modal title={lesson ? "Edit class" : "Schedule a class"} close={close}>
      {!lesson && (
        <div className="academy-schedule-steps" aria-label="Scheduling steps">
          <span aria-current={!preview ? "step" : undefined}>
            <b>1</b> Class details
          </span>
          <Icon name="right" size={16} />
          <span aria-current={preview ? "step" : undefined}>
            <b>2</b> Review & confirm
          </span>
        </div>
      )}
      <form
        hidden={!!preview}
        className="academy-form"
        onChange={() => setPreview(null)}
        onSubmit={async (event) => {
          const form = formValues(event);
          setError("");
          setBusy(true);
          const start = value(form, "localStart");
          const until = value(form, "recurringUntil");
          const input: ClassInput = {
            batchId,
            title: value(form, "title"),
            localStart: start,
            timeZone: value(form, "timeZone"),
            durationMinutes: Number(form.get("durationMinutes")),
            occurrences: weekly
              ? Math.floor(
                  (Date.parse(until) - Date.parse(start.slice(0, 10))) /
                    604800000,
                ) + 1
              : 1,
            ...(weekly ? { recurringUntil: until } : {}),
            meetingUrl: value(form, "meetingUrl"),
            idempotencyKey: key,
            ...(lesson
              ? {
                  revision: lesson.revision,
                  scope: value(form, "scope") as "one" | "future",
                }
              : {}),
          };
          try {
            if (lesson) {
              if (
                await mutate(
                  `/academy/classes/${lesson.id}`,
                  input,
                  "Class updated.",
                  "PATCH",
                )
              ) {
                saved(input, lesson.id);
                close();
              }
            } else {
              const result = await apiFetch<{ classes: AcademyClass[] }>(
                "/academy/classes/preview",
                { method: "POST", body: JSON.stringify(input) },
              );
              setPreview({ input, classes: result.classes });
            }
          } catch (cause) {
            setError(
              cause instanceof Error
                ? cause.message
                : "The schedule could not be checked.",
            );
          } finally {
            setBusy(false);
          }
        }}
      >
        <Field label="Batch">
          <select
            name="batchId"
            value={batchId}
            onChange={(event) => setBatch(event.target.value)}
            required
          >
            {data.batches.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </select>
        </Field>
        {!data.batches.length && (
          <Empty>
            Create a batch and enroll its learners before scheduling a class.
          </Empty>
        )}
        {batch && (
          <p>
            {data.coaches.find((coach) => coach.id === batch.coachId)?.name} ·{" "}
            {batch.studentIds.length} learners · {label(batch.format)}
          </p>
        )}
        <Field label="Lesson title or topic">
          <input
            name="title"
            required
            maxLength={200}
            defaultValue={lesson?.title}
            placeholder="Forcing moves and candidate moves"
          />
        </Field>
        <div className="academy-form-grid">
          <Field label="Local start">
            <input
              name="localStart"
              type="datetime-local"
              required
              defaultValue={
                lesson
                  ? localInput(lesson.startsAt, lesson.timeZone)
                  : undefined
              }
            />
          </Field>
          <Field label="Duration (minutes)">
            <input
              name="durationMinutes"
              type="number"
              min={15}
              max={240}
              required
              defaultValue={
                lesson
                  ? (Date.parse(lesson.endsAt) - Date.parse(lesson.startsAt)) /
                    60000
                  : 60
              }
            />
          </Field>
        </div>
        <Field label="Timezone">
          <input
            name="timeZone"
            key={batchId}
            required
            defaultValue={lesson?.timeZone || batch?.timeZone || "Asia/Kolkata"}
            placeholder="Asia/Kolkata"
          />
        </Field>
        <Field label="Zoom or Meet link">
          <input
            name="meetingUrl"
            type="url"
            maxLength={2000}
            defaultValue={lesson?.meetingUrl}
            placeholder="https://meet.google.com/…"
          />
        </Field>
        {lesson && (
          <Field label="Apply changes to">
            <select name="scope">
              <option value="one">Only this class</option>
              <option value="future">
                This and future classes in this schedule
              </option>
            </select>
          </Field>
        )}
        {!lesson && (
          <label className="academy-check">
            <input
              type="checkbox"
              checked={weekly}
              onChange={(event) => setWeekly(event.target.checked)}
            />
            Repeat weekly
          </label>
        )}
        {weekly && (
          <Field label="Repeat through">
            <input name="recurringUntil" type="date" required />
            <small>Classes keep the same local weekday and time.</small>
          </Field>
        )}
        {error && (
          <p role="alert" className="form-message form-message-error">
            {error}
          </p>
        )}
        <button className="button button-dark" disabled={busy || !batchId}>
          {busy ? "Checking…" : lesson ? "Save changes" : "Preview schedule"}
        </button>
      </form>
      {preview && (
        <div className="academy-preview">
          <h3 ref={reviewHeading} tabIndex={-1}>
            {preview.classes.length} class
            {preview.classes.length === 1 ? "" : "es"} ready to schedule
          </h3>
          <ol>
            {preview.classes.map((item, index) => (
              <li key={index}>{time(item.startsAt, preview.input.timeZone)}</li>
            ))}
          </ol>
          <p>
            <strong>{preview.input.title}</strong> · {batch?.name}
          </p>
          <p>
            {preview.input.meetingUrl ||
              "No meeting link yet. You can add one from class details."}
          </p>
          <p>
            Capacity, overlapping classes, and available class credits will be
            checked again when you confirm.
          </p>
          <div className="academy-actions">
            <button
              type="button"
              disabled={busy}
              onClick={() => setPreview(null)}
            >
              Back to details
            </button>
            <button
              className="button button-dark"
              disabled={busy}
              onClick={async () => {
                setBusy(true);
                if (
                  await mutate(
                    "/academy/classes",
                    preview.input,
                    "Classes scheduled. Learners can now see them in their calendar.",
                  )
                ) {
                  saved(preview.input);
                  close();
                }
                setBusy(false);
              }}
            >
              Confirm schedule
            </button>
          </div>
        </div>
      )}
    </Modal>
  );
}

export default function CalendarPanel({
  data,
  mutate,
  canJoin,
  canCancel,
  compact = false,
  page = "calendar",
}: {
  data: AcademyWorkspaceData;
  mutate: AcademyMutation;
  canJoin: boolean;
  canCancel: boolean;
  compact?: boolean;
  page?: "calendar" | "classes";
}) {
  const [timeZone, setTimeZone] = useState(
    () =>
      data.profile.timeZone ||
      Intl.DateTimeFormat().resolvedOptions().timeZone ||
      "Asia/Kolkata",
  );
  const [focusRequest, setFocusRequest] = useState<{
    input: ClassInput;
    id?: string;
    revision: number;
  } | null>(null);
  const focusedClass = focusRequest
    ? data.classes.find((item) =>
        focusRequest.id
          ? item.id === focusRequest.id
          : item.batchId === focusRequest.input.batchId &&
            item.title === focusRequest.input.title &&
            localInput(item.startsAt, focusRequest.input.timeZone) ===
              focusRequest.input.localStart,
      )
    : undefined;
  const [editor, setEditor] = useState<AcademyClass | "new" | null>(null);
  const [completing, setCompleting] = useState<AcademyClass | null>(null);
  const [cancelling, setCancelling] = useState<AcademyClass | null>(null);
  const [cancelPreview, setCancelPreview] =
    useState<CancellationPreview | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [fulfilling, setFulfilling] = useState<
    AcademyWorkspaceData["makeUps"][number] | null
  >(null);
  const studentName = (id: string) =>
    data.students.find((item) => item.id === id)?.name || "Learner";
  return (
    <>
      <CalendarView
        key={focusRequest?.revision || 0}
        focus={focusedClass}
        data={data}
        page={page}
        compact={compact}
        timeZone={timeZone}
        setTimeZone={setTimeZone}
        canJoin={canJoin}
        canCancel={canCancel}
        onCreate={() => setEditor("new")}
        onEdit={setEditor}
        onComplete={setCompleting}
        onCancel={(item) => {
          setCancelling(item);
          setCancelPreview(null);
          setError("");
        }}
      />
      {editor && (
        <ClassEditor
          data={data}
          lesson={editor === "new" ? undefined : editor}
          close={() => setEditor(null)}
          mutate={mutate}
          saved={(input, id) =>
            setFocusRequest((previous) => ({
              input,
              id,
              revision: (previous?.revision || 0) + 1,
            }))
          }
        />
      )}
      {!compact && data.trialSlots.length > 0 && (
        <section className="workspace-panel" aria-label="Trial schedule">
          <h3>Trial lessons</h3>
          {data.trialSlots.map((slot) => (
            <article className="academy-list-row" key={slot.id}>
              <div>
                <Badge>{label(slot.status)}</Badge>
                <h4>{time(slot.startsAt, timeZone)}</h4>
                <p>
                  {data.coaches.find((coach) => coach.id === slot.coachId)
                    ?.name || "Coach"}{" "}
                  · {slot.capacity - slot.availableSeats} bookings
                </p>
              </div>
            </article>
          ))}
        </section>
      )}
      {!compact && data.makeUps.length > 0 && (
        <section className="workspace-panel" aria-label="Make-up lessons">
          <h3>Make-up lessons</h3>
          {data.makeUps.map((item) => (
            <article className="academy-list-row" key={item.id}>
              <div>
                <Badge>{label(item.status)}</Badge>
                <h4>{studentName(item.studentId)}</h4>
                <p>{label(item.reason)}</p>
                {item.replacementClassId && (
                  <p>
                    Replacement:{" "}
                    {data.classes.find(
                      (lesson) => lesson.id === item.replacementClassId,
                    )?.title || "Scheduled class"}
                  </p>
                )}
              </div>
              {item.status === "open" && data.capabilities.manageScheduling && (
                <button onClick={() => setFulfilling(item)}>
                  Link replacement class
                </button>
              )}
            </article>
          ))}
        </section>
      )}
      {fulfilling && (
        <Modal title="Link a make-up lesson" close={() => setFulfilling(null)}>
          <form
            className="academy-form"
            onSubmit={async (event) => {
              const form = formValues(event);
              if (
                await mutate(
                  `/academy/make-ups/${fulfilling.id}/fulfill`,
                  {
                    replacementClassId: value(form, "replacementClassId"),
                    idempotencyKey: crypto.randomUUID(),
                  },
                  "Make-up linked to its replacement class.",
                )
              )
                setFulfilling(null);
            }}
          >
            <p>
              Select a scheduled class for {studentName(fulfilling.studentId)}.
              The replacement uses its reserved credit; this does not grant
              another credit.
            </p>
            <Field label="Replacement class">
              <select name="replacementClassId" required defaultValue="">
                <option value="" disabled>
                  Select a class
                </option>
                {data.classes
                  .filter(
                    (lesson) =>
                      lesson.status === "scheduled" &&
                      lesson.studentIds.includes(fulfilling.studentId) &&
                      Date.parse(lesson.startsAt) > Date.now() &&
                      !data.makeUps.some(
                        (item) =>
                          item.studentId === fulfilling.studentId &&
                          item.replacementClassId === lesson.id,
                      ),
                  )
                  .map((lesson) => (
                    <option key={lesson.id} value={lesson.id}>
                      {lesson.title} · {time(lesson.startsAt, timeZone)}
                    </option>
                  ))}
              </select>
            </Field>
            <button className="button button-dark">Confirm replacement</button>
          </form>
        </Modal>
      )}
      {completing && (
        <Modal
          title="Attendance & lesson notes"
          close={() => setCompleting(null)}
        >
          <form
            className="academy-form"
            onSubmit={async (event) => {
              const form = formValues(event);
              setBusy(true);
              if (
                await mutate(
                  `/academy/classes/${completing.id}/complete`,
                  {
                    revision: completing.revision,
                    notes: value(form, "notes"),
                    attendance: Object.fromEntries(
                      completing.studentIds.map((id) => [id, value(form, id)]),
                    ),
                    idempotencyKey: crypto.randomUUID(),
                  },
                  "Class completed. Attendance and notes are saved.",
                )
              )
                setCompleting(null);
              setBusy(false);
            }}
          >
            <h3>{completing.title}</h3>
            {completing.studentIds.map((id) => (
              <Field key={id} label={studentName(id)}>
                <select
                  name={id}
                  required
                  defaultValue={completing.attendance[id] || ""}
                >
                  <option value="" disabled>
                    Mark attendance
                  </option>
                  <option value="on_time">Present</option>
                  <option value="late">Late</option>
                  <option value="excused">Excused absence</option>
                  <option value="unexcused">Absent</option>
                  <option value="technical_failure">Technical failure</option>
                </select>
              </Field>
            ))}
            <Field label="Lesson notes">
              <textarea
                name="notes"
                required
                minLength={10}
                maxLength={3000}
                defaultValue={completing.notes}
              />
            </Field>
            <button className="button button-dark" disabled={busy}>
              Complete class
            </button>
          </form>
        </Modal>
      )}
      {cancelling && (
        <Modal title="Cancellation options" close={() => setCancelling(null)}>
          <form
            className="academy-form"
            onChange={() => setCancelPreview(null)}
            onSubmit={async (event) => {
              const form = formValues(event);
              setBusy(true);
              setError("");
              try {
                const input = {
                  revision: cancelling.revision,
                  studentId: value(form, "studentId"),
                  reason: value(form, "reason"),
                  idempotencyKey: crypto.randomUUID(),
                };
                if (cancelPreview) {
                  if (
                    await mutate(
                      `/academy/classes/${cancelling.id}/cancel`,
                      input,
                      "Cancellation saved.",
                    )
                  )
                    setCancelling(null);
                } else
                  setCancelPreview(
                    await apiFetch<CancellationPreview>(
                      `/academy/classes/${cancelling.id}/cancel-preview`,
                      { method: "POST", body: JSON.stringify(input) },
                    ),
                  );
              } catch (cause) {
                setError(
                  cause instanceof Error
                    ? cause.message
                    : "Cancellation could not be checked.",
                );
              } finally {
                setBusy(false);
              }
            }}
          >
            <h3>{cancelling.title}</h3>
            <Field label="Affected learner">
              <select name="studentId">
                {data.capabilities.manageScheduling && (
                  <option value="">Academy cancellation — entire class</option>
                )}
                {cancelling.studentIds.map((id) => (
                  <option key={id} value={id}>
                    {studentName(id)}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="Reason">
              <textarea name="reason" required minLength={5} maxLength={1000} />
            </Field>
            {cancelPreview && (
              <div className="academy-preview">
                {cancelPreview.outcomes.map((item) => (
                  <p key={item.studentId}>
                    <strong>{studentName(item.studentId)}</strong>:{" "}
                    {label(item.action)} · {label(item.reason)}
                    {item.makeUp && " · Make-up owed"}
                  </p>
                ))}
              </div>
            )}
            {error && <p role="alert">{error}</p>}
            <button className="button button-dark" disabled={busy}>
              {cancelPreview ? "Confirm cancellation" : "Preview outcome"}
            </button>
          </form>
        </Modal>
      )}
    </>
  );
}
