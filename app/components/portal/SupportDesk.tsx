"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Icon } from "@/app/components/Icon";
import { APIError } from "@/app/lib/api/client";
import { isDemoMode } from "@/app/lib/demo/mode";
import { fetchWorkspace } from "@/app/lib/demo/client";
import {
  appendSupportCaseEvent,
  assignSupportCase,
  changeSupportCaseStatus,
  createSupportCase,
  getSupportCase,
  listSupportCases,
  type CreateSupportCase,
  type SupportCase,
} from "@/app/lib/api/typed-client";

const UUID_V7 = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

const STATUS_TRANSITIONS: Record<SupportCase["status"], SupportCase["status"][]> = {
  open: ["investigating", "waiting", "resolved"],
  investigating: ["waiting", "resolved"],
  waiting: ["investigating", "resolved"],
  resolved: ["investigating", "closed"],
  closed: ["investigating"],
};

function messageFor(error: unknown) {
  return error instanceof APIError ? error.message : "The support service could not complete that request.";
}

function displayDate(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

export default function SupportDesk({ canManage }: { canManage: boolean }) {
  const [familyInput, setFamilyInput] = useState("");
  const [familyId, setFamilyId] = useState("");
  const [cases, setCases] = useState<SupportCase[]>([]);
  const [selected, setSelected] = useState<SupportCase | null>(null);
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [familyName, setFamilyName] = useState("");

  const loadFamily = useCallback(async (targetFamilyId: string, cursor?: string) => {
    if (!UUID_V7.test(targetFamilyId)) {
      setError("Enter the UUIDv7 family reference shown in the academy family record.");
      return;
    }
    setLoading(true);
    setError("");
    setNotice("");
    try {
      const page = await listSupportCases(targetFamilyId, cursor);
      setFamilyId(targetFamilyId);
      setFamilyInput(targetFamilyId);
      setCases((current) => (cursor ? [...current, ...page.items] : page.items));
      setNextCursor(page.nextCursor);
      if (!cursor) setSelected(null);
      const url = new URL(window.location.href);
      url.searchParams.set("familyId", targetFamilyId);
      window.history.replaceState(null, "", url);
    } catch (requestError) {
      setError(messageFor(requestError));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const value = new URLSearchParams(window.location.search).get("familyId")?.trim() ?? "";
    if (UUID_V7.test(value)) void loadFamily(value);
    else if (value) setFamilyInput(value);
    else if (isDemoMode()) {
      void fetchWorkspace().then((workspace) => {
        setFamilyName(workspace.family.name);
        return loadFamily(workspace.family.id);
      }).catch((requestError) => setError(messageFor(requestError)));
    }
  }, [loadFamily]);

  function replaceCase(updated: SupportCase) {
    setSelected(updated);
    setCases((current) => current.map((item) => (item.id === updated.id ? updated : item)));
  }

  async function selectCase(caseId: string) {
    setLoading(true);
    setError("");
    setNotice("");
    try {
      setSelected(await getSupportCase(caseId));
    } catch (requestError) {
      setError(messageFor(requestError));
    } finally {
      setLoading(false);
    }
  }

  async function submitFamily(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await loadFamily(familyInput.trim());
  }

  async function createCase(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!familyId) return;
    const form = event.currentTarget;
    const data = new FormData(form);
    setSaving(true);
    setError("");
    setNotice("");
    try {
      const created = await createSupportCase({
        familyId,
        category: String(data.get("category")) as CreateSupportCase["category"],
        severity: String(data.get("severity")) as CreateSupportCase["severity"],
        subject: String(data.get("subject") ?? "").trim(),
        message: String(data.get("message") ?? "").trim(),
      });
      setCases((current) => [created, ...current]);
      setSelected(created);
      setNotice("Your support case is open. Replies in this timeline are visible to the family.");
      form.reset();
    } catch (requestError) {
      setError(messageFor(requestError));
    } finally {
      setSaving(false);
    }
  }

  async function reply(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected) return;
    const form = event.currentTarget;
    const data = new FormData(form);
    setSaving(true);
    setError("");
    setNotice("");
    try {
      const updated = await appendSupportCaseEvent(selected.id, {
        version: selected.version,
        visibility: canManage && data.get("visibility") === "internal" ? "internal" : "customer_visible",
        body: String(data.get("body") ?? "").trim(),
      });
      replaceCase(updated);
      setNotice(data.get("visibility") === "internal" ? "Internal note added." : "Reply added to the family-visible timeline.");
      form.reset();
    } catch (requestError) {
      setError(messageFor(requestError));
    } finally {
      setSaving(false);
    }
  }

  async function changeStatus(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected || !canManage) return;
    const data = new FormData(event.currentTarget);
    setSaving(true);
    setError("");
    setNotice("");
    try {
      const updated = await changeSupportCaseStatus(selected.id, {
        version: selected.version,
        status: String(data.get("status")) as "open" | "investigating" | "waiting" | "resolved" | "closed",
        reason: String(data.get("reason") ?? "").trim(),
      });
      replaceCase(updated);
      setNotice("Case status updated with audit evidence.");
    } catch (requestError) {
      setError(messageFor(requestError));
    } finally {
      setSaving(false);
    }
  }

  async function assign(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected || !canManage) return;
    const data = new FormData(event.currentTarget);
    setSaving(true);
    setError("");
    setNotice("");
    try {
      const updated = await assignSupportCase(selected.id, {
        version: selected.version,
        assigneeId: String(data.get("assigneeId") ?? "").trim(),
        reason: String(data.get("reason") ?? "").trim(),
      });
      replaceCase(updated);
      setNotice("Case assignment updated with audit evidence.");
    } catch (requestError) {
      setError(messageFor(requestError));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="support-workspace" aria-busy={loading || saving}>
      <section className="support-reference-card" aria-labelledby="family-reference-heading">
        <div>
          <span className="eyebrow">Here to help</span>
          <h2 id="family-reference-heading">{isDemoMode() ? familyName || "Family support" : "Open your family support"}</h2>
          <p>Questions about a class, your account, or learning? Keep the conversation with your academy in one place.</p>
        </div>
        {!isDemoMode() && <form onSubmit={submitFamily} className="support-reference-form">
          <label htmlFor="support-family-id">Family reference</label>
          <div>
            <input
              id="support-family-id"
              value={familyInput}
              onChange={(event) => setFamilyInput(event.target.value)}
              placeholder="Family reference from the academy"
              autoComplete="off"
              required
            />
            <button className="button button-dark" disabled={loading}>{loading ? "Loading…" : "Open"}</button>
          </div>
        </form>}
      </section>

      {error && <p className="form-message form-message-error" role="alert">{error}</p>}
      {notice && <p className="form-message support-notice" role="status">{notice}</p>}

      {familyId && (
        <div className="support-columns">
          <div className="support-list-column">
            <section className="support-panel">
              <div className="support-panel-heading">
                <div><span className="eyebrow">Family support</span><h2>Cases</h2></div>
                <span>{cases.length}</span>
              </div>
              {cases.length === 0 && !loading ? (
                <p className="support-empty">No support cases are open for this family record.</p>
              ) : (
                <div className="support-case-list">
                  {cases.map((supportCase) => (
                    <button
                      key={supportCase.id}
                      type="button"
                      className={selected?.id === supportCase.id ? "is-selected" : ""}
                      aria-pressed={selected?.id === supportCase.id}
                      onClick={() => void selectCase(supportCase.id)}
                    >
                      <span><strong>{supportCase.subject}</strong><small>{supportCase.category} · {supportCase.severity}</small></span>
                      <em>{supportCase.status}</em>
                    </button>
                  ))}
                </div>
              )}
              {nextCursor && <button className="support-load-more" disabled={loading} onClick={() => void loadFamily(familyId, nextCursor)}>Load earlier cases</button>}
            </section>

            <section className="support-panel">
              <span className="eyebrow">New request</span>
              <h2>Ask the academy for help</h2>
              <form className="support-form" onSubmit={createCase}>
                <label>Category<select name="category" defaultValue="class"><option value="class">Class</option><option value="learning">Learning</option><option value="account">Account</option><option value="policy">Policy</option><option value="technical">Technical</option><option value="other">Other</option></select></label>
                <label>Priority<select name="severity" defaultValue="medium"><option value="low">Low</option><option value="medium">Medium</option><option value="high">High</option><option value="critical">Critical</option></select></label>
                <label className="support-wide">Subject<input name="subject" maxLength={200} required /></label>
                <label className="support-wide">Message<textarea name="message" rows={5} maxLength={10000} required /></label>
                <button className="button button-dark support-wide" disabled={saving}>{saving ? "Saving…" : "Open case"}</button>
              </form>
            </section>
          </div>

          <section className="support-panel support-timeline" aria-live="polite">
            {!selected ? (
              <div className="support-empty support-empty-large"><Icon name="message" size={40} /><span>Support timeline</span><p>Select a case to see the conversation, or open a new request.</p></div>
            ) : (
              <>
                <header>
                  <span className="eyebrow">{selected.category} · {selected.severity}</span>
                  <h2>{selected.subject}</h2>
                  <div><span className="support-status">{selected.status}</span><small>Updated {displayDate(selected.updatedAt)}</small></div>
                </header>
                <ol>
                  {selected.events.map((event) => (
                    <li key={event.id} className={event.visibility === "internal" ? "is-internal" : ""}>
                      <div><strong>{event.type.replaceAll("_", " ")}</strong><span>{event.visibility === "internal" ? "Internal" : "Family visible"}</span></div>
                      {event.body && <p>{event.body}</p>}
                      <time dateTime={event.occurredAt}>{displayDate(event.occurredAt)}</time>
                    </li>
                  ))}
                </ol>
                <form className="support-reply" onSubmit={reply}>
                  <label htmlFor="support-reply">Add a reply</label>
                  <textarea id="support-reply" name="body" rows={4} maxLength={10000} required />
                  {canManage && <label className="support-check"><input type="checkbox" name="visibility" value="internal" /> Save as an internal note</label>}
                  <button className="button button-dark" disabled={saving}>{saving ? "Saving…" : "Add reply"}</button>
                </form>
                {canManage && (
                  <div className="support-operator-tools">
                    <form key={`status-${selected.id}-${selected.version}`} onSubmit={changeStatus}>
                      <h3>Change status</h3>
                      <label>Status<select name="status" defaultValue={STATUS_TRANSITIONS[selected.status][0]}>{STATUS_TRANSITIONS[selected.status].map((status) => <option key={status} value={status}>{status.replaceAll("_", " ")}</option>)}</select></label>
                      <label>Reason<input name="reason" maxLength={500} required /></label>
                      <button disabled={saving}>Update status</button>
                    </form>
                    <form onSubmit={assign}>
                      <h3>Assign case</h3>
                      <label>Assign to{isDemoMode() ? <select name="assigneeId"><option value="demo-operations_admin">Alex · Operations</option><option value="demo-super_admin">Admin</option></select> : <input name="assigneeId" placeholder="Staff reference" pattern={UUID_V7.source} required />}</label>
                      <label>Reason<input name="reason" maxLength={500} required /></label>
                      <button disabled={saving}>Assign</button>
                    </form>
                  </div>
                )}
              </>
            )}
          </section>
        </div>
      )}
    </div>
  );
}
