"use client";

import { useCallback, useEffect, useState } from "react";
import { apiFetch } from "@/app/lib/api/client";
import { staffRoles, type AcademyMutation, type AcademyStaff, type AcademyStaffDirectory } from "@/app/lib/academy-types";
import type { AcademyRole } from "@/app/lib/api/types";
import { roleLabels } from "@/app/lib/role-routing";
import { Badge, Empty, Field, Modal, Panel, formValues, value } from "./WorkspaceUI";

export default function StaffAccess({ actorId, mutate }: {
  actorId: string; mutate: AcademyMutation;
}) {
  const [staff, setStaff] = useState<AcademyStaff[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [query, setQuery] = useState("");
  const [editing, setEditing] = useState<AcademyStaff | null>(null);
  const load = useCallback(async () => {
    setLoading(true);
    try { const result = await apiFetch<AcademyStaffDirectory>("/academy/staff"); setStaff(result.staff); setError(""); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "The staff directory could not be loaded."); }
    finally { setLoading(false); }
  }, []);
  useEffect(() => { void load(); }, [load]);
  const shown = staff.filter((person) => `${person.displayName} ${person.email} ${person.roles.join(" ")} ${person.status}`.toLowerCase().includes(query.toLowerCase()));

  return <Panel title="Staff access" action={<button disabled={loading || busy} onClick={() => void load()}>Refresh staff</button>}>
    <p>Review staff roles and account access. Suspending an account signs out all its remembered browsers. Reactivated staff must sign in again.</p>
    {error && !editing && <p role="alert" className="form-message form-message-error">{error}</p>}
    {notice && <p role="status" className="form-message">{notice}</p>}
    <Field label="Search staff"><input type="search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Name, email, role or status" /></Field>
    {loading ? <p role="status">Loading staff…</p> : shown.length ? <div className="academy-stack">{shown.map((person) => <article className="academy-list-row" key={person.id}>
      <div><h3>{person.displayName}{person.id === actorId ? " (you)" : ""}</h3><p>{person.email || "No email recorded"}</p><p>{person.roles.map((role) => roleLabels[role as AcademyRole] || role.replaceAll("_", " ")).join(" · ")}</p><Badge>{person.status}</Badge></div>
      <button disabled={busy || !["active", "suspended"].includes(person.status)} onClick={() => { setError(""); setEditing(person); }}>Edit staff access</button>
    </article>)}</div> : <Empty>{query ? "No staff match this search." : "No staff accounts found."}</Empty>}
    <details><summary>Create a staff account</summary><form className="academy-form" onSubmit={async (event) => {
      const form = event.currentTarget; const fields = formValues(event); setBusy(true);
      try { if (await mutate("/academy/staff", { name: value(fields, "name"), email: value(fields, "email"), role: value(fields, "role") }, "Staff account created. The user can sign in using their academy email and set up MFA.")) { form.reset(); await load(); } }
      finally { setBusy(false); }
    }}><Field label="Staff name"><input name="name" required minLength={2} maxLength={100} /></Field><Field label="Staff email"><input name="email" type="email" required /></Field><Field label="Role"><select name="role">{staffRoles.map((role) => <option key={role} value={role}>{roleLabels[role]}</option>)}</select></Field><button className="button button-dark" disabled={busy}>Create staff account</button></form></details>
    {editing && <Modal title="Edit staff access" close={() => { if (!busy) setEditing(null); }}><form className="academy-form" onSubmit={async (event) => {
      const form = formValues(event); setBusy(true); setError(""); setNotice("");
      try {
        const message = value(form, "status") === "suspended" ? "Staff access suspended and all remembered browsers signed out." : "Staff details saved.";
        if (await mutate(`/academy/staff/${editing.id}`, { displayName: value(form, "displayName"), status: value(form, "status"), expectedUpdatedAt: editing.updatedAt }, message, "PATCH")) {
          setEditing(null); setNotice(message); await load();
        }
      } catch (cause) { setError(cause instanceof Error ? cause.message : "Staff access could not be saved."); }
      finally { setBusy(false); }
    }}><p>{editing.email} · Roles: {editing.roles.map((role) => roleLabels[role as AcademyRole] || role).join(", ")}</p>
      <Field label="Staff display name"><input name="displayName" required minLength={2} maxLength={100} defaultValue={editing.displayName} disabled={busy} /></Field>
      <Field label="Account status"><select name="status" defaultValue={editing.status} disabled={busy}><option value="active">Active</option><option value="suspended" disabled={editing.id === actorId}>Suspended</option></select></Field>
      {editing.id === actorId && <p>Another academy owner must suspend your account.</p>}
      {error && <p role="alert" className="form-message form-message-error">{error}</p>}
      <button className="button button-dark" disabled={busy}>{busy ? "Saving…" : "Save staff access"}</button>
    </form></Modal>}
  </Panel>;
}
