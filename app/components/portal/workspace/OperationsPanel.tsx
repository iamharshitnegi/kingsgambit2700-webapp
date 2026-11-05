"use client";

import { useCallback, useEffect, useState } from "react";
import { apiFetch } from "@/app/lib/api/client";
import { requestID } from "@/app/lib/request-id";
import type { AcademyLead, AcademyMutation, AcademyOperationsData, AcademyTrialBooking, AcademyWorkspaceData } from "@/app/lib/academy-types";
import { Badge, Empty, Field, Modal, Panel, formValues, label, time, value } from "./WorkspaceUI";

const stages = ["new", "contacted", "trial_booked", "trial_attended", "assessed", "offered", "unqualified", "lost", "duplicate", "do_not_contact"];

export default function OperationsPanel({ data, mutate }: { data: AcademyWorkspaceData; mutate: AcademyMutation }) {
  const [operations, setOperations] = useState<AcademyOperationsData | null>(null);
  const [limit, setLimit] = useState(500);
  const [error, setError] = useState("");
  const [search, setSearch] = useState("");
  const [busy, setBusy] = useState(false);
  const [lead, setLead] = useState<AcademyLead | null>(null);
  const [conversion, setConversion] = useState<AcademyLead | null>(null);
  const [trial, setTrial] = useState<AcademyTrialBooking | null>(null);
  const [rescheduling, setRescheduling] = useState<AcademyTrialBooking | null>(null);
  const [slotOpen, setSlotOpen] = useState(false);
  const [format, setFormat] = useState("group");
  const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const load = useCallback(async () => {
    if (!data.capabilities.manageAcademy) return;
    try { setOperations(await apiFetch<AcademyOperationsData>(`/academy/operations${limit === 500 ? "" : `?limit=${limit}`}`)); setError(""); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "Academy operations could not be loaded."); }
  }, [data.capabilities.manageAcademy, limit]);
  useEffect(() => { void load(); }, [load]);

  async function save(path: string, input: Record<string, unknown>, message: string, method: "POST" | "PATCH" = "POST") {
    setBusy(true);
    try { const saved = await mutate(path, input, message, method); if (saved) await load(); return saved; }
    finally { setBusy(false); }
  }

  if (!data.capabilities.manageAcademy) return <Empty>Your role does not manage enquiries and trial bookings.</Empty>;
  if (!operations) return <Panel title="Academy operations">{error ? <><p role="alert">{error}</p><button onClick={() => void load()}>Try again</button></> : <p role="status">Loading enquiries and trials…</p>}</Panel>;
  const filtered = operations.leads.filter((item) => `${item.guardianName} ${item.email} ${item.status}`.toLowerCase().includes(search.toLowerCase()));
  const pagination = (operations as AcademyOperationsData & { pagination?: { hasMore: boolean; nextLimit: number } }).pagination;

  return <div className="academy-stack">
    {pagination?.hasMore && <Panel title="More academy records available"><p>Search covers the records currently loaded. Load more to include older enquiries and trials.</p><button disabled={busy} onClick={() => setLimit(pagination.nextLimit)}>Load more operations records</button></Panel>}
    {error && <p role="alert" className="form-message form-message-error">{error}</p>}
    <Panel title="Enquiries & next steps" action={<button disabled={busy} onClick={() => void load()}>Refresh operations</button>}>
      <Field label="Find an enquiry"><input type="search" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Family, email or stage" /></Field>
      {filtered.length ? <div className="academy-card-grid">{filtered.map((item) => <article className="academy-record" key={item.id}>
        <Badge>{label(item.status)}</Badge><h3>{item.guardianName}</h3><p>{item.email || item.phone}</p><p>{label(item.ageBand)} · {time(item.createdAt, data.profile?.timeZone)}</p>{item.notes && <p className="academy-prose">{item.notes}</p>}
        {item.studentId ? <p>Enrolled learner: {data.students.find((student) => student.id === item.studentId)?.name || "Linked learner"}</p> : <div className="academy-actions"><button onClick={() => setLead(item)}>Update enquiry</button><button onClick={() => setConversion(item)}>Link enrolled learner</button></div>}
      </article>)}</div> : <Empty>{search ? "No enquiries match your search." : "New family enquiries appear here after they contact the academy or book a trial."}</Empty>}
    </Panel>
    <Panel title="Trial availability" action={<button className="button button-dark" onClick={() => setSlotOpen(true)}>Schedule a trial slot</button>}>
      {operations.trialSlots.length ? <div className="academy-card-grid">{operations.trialSlots.map((slot) => <article className="academy-record" key={slot.id}><Badge>{label(slot.status)}</Badge><h3>{time(slot.startsAt, slot.timeZone)}</h3><p>{data.coaches.find((coach) => coach.id === slot.coachId)?.name || "Academy coach"} · {label(slot.format)}</p><p>{slot.availableSeats} of {slot.capacity} seats available · {slot.timeZone}</p><p>Ages {slot.ageBands.map(label).join(", ")}</p></article>)}</div> : <Empty>Publish trial slots so families can book an available assessment time.</Empty>}
    </Panel>
    <Panel title="Trial bookings & outcomes">
      {operations.trialBookings.length ? <div className="academy-table-scroll" tabIndex={0} role="region" aria-label="Trial bookings"><table className="academy-table"><caption>Booked assessments</caption><thead><tr><th>Family</th><th>When</th><th>Outcome</th><th>Next action</th></tr></thead><tbody>{operations.trialBookings.map((item) => <tr key={item.id}><td>{item.guardianName}{item.studentName && <small>{item.studentName}</small>}</td><td>{time(item.startsAt, data.profile?.timeZone)}</td><td>{label(item.status)}{item.notes && <p>{item.notes}</p>}</td><td><div className="academy-actions"><button onClick={() => setTrial(item)}>Update trial</button>{item.status === "confirmed" && Date.parse(item.startsAt) > Date.now() && <button onClick={() => setRescheduling(item)}>Reschedule trial</button>}</div></td></tr>)}</tbody></table></div> : <Empty>Confirmed public trial bookings appear here automatically.</Empty>}
    </Panel>
    {lead && <Modal title={`Follow up with ${lead.guardianName}`} close={() => setLead(null)}><form className="academy-form" onSubmit={async (event) => { const form = formValues(event); if (await save(`/academy/leads/${lead.id}`, { status: value(form, "status"), notes: value(form, "notes") }, "Enquiry updated.", "PATCH")) setLead(null); }}><Field label="Enquiry stage"><select name="status" defaultValue={lead.status}>{stages.map((stage) => <option key={stage} value={stage}>{label(stage)}</option>)}</select></Field><Field label="Follow-up notes"><textarea name="notes" rows={4} maxLength={5000} defaultValue={lead.notes} /></Field><button className="button button-dark" disabled={busy}>Save follow-up</button></form></Modal>}
    {conversion && <Modal title={`Enroll ${conversion.guardianName}'s family`} close={() => setConversion(null)}><p>Add the learner and their guardian, record consent and create an enrollment in Learners & batches. Then link that learner here to complete the enquiry.</p><p>The guardian email must match {conversion.email || "a verified email on this enquiry"}.</p><form className="academy-form" onSubmit={async (event) => { const form = formValues(event); if (await save(`/academy/leads/${conversion.id}/convert`, { studentId: value(form, "studentId") }, "Enquiry linked to the enrolled learner.")) setConversion(null); }}><Field label="Enrolled learner"><select name="studentId" required defaultValue=""><option value="" disabled>Select a learner</option>{data.students.filter((student) => data.enrolments.some((enrolment) => enrolment.studentId === student.id && enrolment.status === "active")).map((student) => <option key={student.id} value={student.id}>{student.name}</option>)}</select></Field><button className="button button-dark" disabled={busy}>Complete enrollment link</button></form></Modal>}
    {trial && <Modal title={`Trial for ${trial.guardianName}`} close={() => setTrial(null)}><form className="academy-form" onSubmit={async (event) => { const form = formValues(event); if (await save(`/academy/trial-bookings/${trial.id}`, { status: value(form, "status"), notes: value(form, "notes") }, "Trial outcome saved.", "PATCH")) setTrial(null); }}><Field label="Trial status"><select name="status" defaultValue={trial.status}>{["confirmed", "attended", "no_show", "cancelled"].map((status) => <option key={status} value={status}>{label(status)}</option>)}</select></Field><Field label="Assessment and follow-up notes"><textarea name="notes" rows={4} maxLength={5000} defaultValue={trial.notes} /></Field><button className="button button-dark" disabled={busy}>Save trial outcome</button></form></Modal>}
    {rescheduling && <Modal title={`Reschedule ${rescheduling.guardianName}'s trial`} close={() => setRescheduling(null)}><p>Current time: {time(rescheduling.startsAt, data.profile?.timeZone)}. The family and assessors receive an update after the change is saved.</p><form className="academy-form" onSubmit={async (event) => { const form = formValues(event); if (await save(`/academy/trial-bookings/${rescheduling.id}/reschedule`, { slotId: value(form, "slotId"), reason: value(form, "reason"), idempotencyKey: requestID() }, "Trial rescheduled and notifications queued.")) setRescheduling(null); }}><Field label="New trial slot"><select name="slotId" required defaultValue=""><option value="" disabled>Select another available slot</option>{operations.trialSlots.filter((slot) => slot.id !== rescheduling.slotId && slot.status === "published" && slot.availableSeats > 0 && Date.parse(slot.startsAt) > Date.now()).map((slot) => <option key={slot.id} value={slot.id}>{time(slot.startsAt, slot.timeZone)} · {data.coaches.find((coach) => coach.id === slot.coachId)?.name || "Coach"} · {slot.availableSeats} seats</option>)}</select></Field><Field label="Reason for the change"><textarea name="reason" minLength={5} maxLength={1000} required /></Field><button className="button button-dark" disabled={busy}>Confirm new trial time</button></form></Modal>}
    {slotOpen && <Modal title="Publish a trial slot" close={() => setSlotOpen(false)}><form className="academy-form" onSubmit={async (event) => { const form = formValues(event); const input = { coachId: value(form, "coachId"), startsAt: new Date(value(form, "startsAt")).toISOString(), endsAt: new Date(value(form, "endsAt")).toISOString(), timeZone: zone, format, capacity: format === "one_to_one" ? 1 : Number(form.get("capacity")), ageBands: form.getAll("ageBands").map(String) }; if (await save("/academy/trial-slots", input, "Trial slot published for family booking.")) setSlotOpen(false); }}>
      <Field label="Trial coach"><select name="coachId" required defaultValue=""><option value="" disabled>Select a coach</option>{data.coaches.map((coach) => <option key={coach.id} value={coach.id}>{coach.name}</option>)}</select></Field><p>Enter times in your device timezone: {zone}.</p><Field label="Trial start"><input name="startsAt" type="datetime-local" required /></Field><Field label="Trial end"><input name="endsAt" type="datetime-local" required /></Field><Field label="Trial format"><select value={format} onChange={(event) => setFormat(event.target.value)}><option value="group">Small group</option><option value="one_to_one">One to one</option></select></Field>{format === "group" && <Field label="Trial seats"><input name="capacity" type="number" min={1} max={6} defaultValue={4} required /></Field>}<fieldset><legend>Student age bands</legend>{["4_7", "8_12", "13_17", "18_plus"].map((age) => <label className="academy-check" key={age}><input name="ageBands" value={age} type="checkbox" defaultChecked={age === "8_12"} />{label(age)}</label>)}</fieldset><button className="button button-dark" disabled={busy || !data.coaches.length}>Publish trial slot</button>
    </form></Modal>}
  </div>;
}
