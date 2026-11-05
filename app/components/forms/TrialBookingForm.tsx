"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { getFieldErrors } from "@/app/lib/api/client";
import { createLead, createTrialBooking, listTrialSlots } from "@/app/lib/api/typed-client";
import { isDemoMode } from "@/app/lib/demo/mode";
import type {
  LeadInput,
  AgeBand,
  TrialFormat,
  TrialBookingInput,
  TrialBookingResponse,
  TrialSlot,
} from "@/app/lib/api/types";

function formatSlot(slot: TrialSlot, timeZone: string) {
  const startsAt = new Date(slot.startsAt);
  return new Intl.DateTimeFormat(undefined, {
    weekday: "short",
    day: "numeric",
    month: "short",
    hour: "numeric",
    minute: "2-digit",
    timeZone,
    timeZoneName: "short",
  }).format(startsAt);
}

export default function TrialBookingForm() {
  const [timeZone, setTimeZone] = useState("Asia/Kolkata");
  const [format, setFormat] = useState<TrialFormat>("group");
  const [ageBand, setAgeBand] = useState<AgeBand>();
  const bookingIdempotencyKey = useRef<string | null>(null);
  const previousBookingInput = useRef("");
  const savedLeadRef = useRef<{ fingerprint: string; id: string } | null>(null);
  const [slots, setSlots] = useState<TrialSlot[]>([]);
  const [slotsLoading, setSlotsLoading] = useState(true);
  const [slotsUnavailable, setSlotsUnavailable] = useState(false);
  const [state, setState] = useState<"idle" | "submitting" | "booked" | "enquired">("idle");
  const [message, setMessage] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [booking, setBooking] = useState<TrialBookingResponse | null>(null);

  const range = useMemo(() => {
    const from = new Date();
    const to = new Date(from.getTime() + 21 * 24 * 60 * 60 * 1000);
    return { from: from.toISOString(), to: to.toISOString() };
  }, []);

  useEffect(() => {
    const detected = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (detected && detected !== "Asia/Kolkata") {
      queueMicrotask(() => {
        setSlotsLoading(true);
        setSlotsUnavailable(false);
        setTimeZone(detected);
      });
    }
  }, []);

  useEffect(() => {
    let active = true;
    const query = {
      timezone: timeZone,
      format,
      ageBand,
      from: range.from,
      to: range.to,
    };
    listTrialSlots(query)
      .then((items) => {
        if (!active) return;
        setSlots(Array.isArray(items) ? items.filter((slot) => slot.availableSeats !== 0) : []);
      })
      .catch(() => {
        if (active) {
          setSlots([]);
          setSlotsUnavailable(true);
        }
      })
      .finally(() => active && setSlotsLoading(false));
    return () => {
      active = false;
    };
  }, [ageBand, format, range.from, range.to, timeZone]);

  function selectFormat(nextFormat: TrialFormat) {
    if (nextFormat === format) return;
    setSlotsLoading(true);
    setSlotsUnavailable(false);
    setFormat(nextFormat);
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const formElement = event.currentTarget;
    const data = new FormData(formElement);
    const params = new URLSearchParams(window.location.search);
    const email = String(data.get("email") || "").trim();
    const phone = String(data.get("phone") || "").trim();
    const guardianName = String(data.get("guardianName") || "").trim();
    const studentAgeBand = String(data.get("studentAgeBand") || "") as AgeBand;
    const slotId = String(data.get("slotId") || "");

    const lead: LeadInput = {
      guardianName,
      email,
      phone: phone || undefined,
      studentAgeBand,
      experience: String(data.get("experience") || "").trim() || undefined,
      rating: Number(data.get("rating")) || undefined,
      country: String(data.get("country") || "").trim().toUpperCase(),
      timeZone,
      preferredFormat: format,
      contactConsent: data.get("contactConsent") === "on",
      marketingConsent: data.get("marketingConsent") === "on",
      source: params.get("utm_source") || "website",
      campaign: params.get("utm_campaign") || undefined,
      medium: params.get("utm_medium") || undefined,
      term: params.get("utm_term") || undefined,
      content: params.get("utm_content") || undefined,
      landingPage: window.location.href,
      referrer: document.referrer || undefined,
    };

    setState("submitting");
    setMessage("");
    setFieldErrors({});
    try {
      const fingerprint = JSON.stringify(lead);
      const savedLead = savedLeadRef.current?.fingerprint === fingerprint
        ? savedLeadRef.current
        : await createLead(lead);
      savedLeadRef.current = { fingerprint, id: savedLead.id };

      if (!slotId) {
        setState("enquired");
        setMessage("Your details are safely recorded. The academy will contact you when a suitable assessment time is available.");
        formElement.reset();
        return;
      }

      const bookingInput: TrialBookingInput = {
        slotId,
        leadId: savedLead.id,
        guardianName,
        email,
        phone: phone || undefined,
        studentName: String(data.get("studentName") || "").trim() || undefined,
        studentAgeBand,
        timeZone,
        contactConsent: true,
      };
      const bookingFingerprint = JSON.stringify(bookingInput);
      if (previousBookingInput.current !== bookingFingerprint) {
        bookingIdempotencyKey.current = null;
        previousBookingInput.current = bookingFingerprint;
      }
      bookingIdempotencyKey.current ||= crypto.randomUUID();
      const confirmed = await createTrialBooking(bookingInput, bookingIdempotencyKey.current);
      bookingIdempotencyKey.current = null;
      setBooking(confirmed);
      setState("booked");
      formElement.reset();
    } catch (error) {
      setFieldErrors(getFieldErrors(error));
      setState("idle");
      setMessage(error instanceof Error ? error.message : "We could not complete this request.");
    }
  }

  if (state === "booked" && booking) {
    return (
      <div className="form-success" role="status">
        <span className="form-success-mark">✓</span>
        <span className="eyebrow">Trial confirmed</span>
        <h2>Your assessment is booked.</h2>
        <p>{formatSlot({ id: booking.id, startsAt: booking.startsAt, endsAt: booking.endsAt, format }, booking.timeZone)}</p>
        <p className="form-fineprint">{isDemoMode() ? "This booking is saved in your local sample academy. No email or message is sent." : "A confirmation will be sent to the contact details you provided."}</p>
      </div>
    );
  }

  if (state === "enquired") {
    return (
      <div className="form-success" role="status">
        <span className="form-success-mark">✓</span>
        <h2>Your trial request is recorded.</h2>
        <p>{message}</p>
      </div>
    );
  }

  return (
    <form className="academy-form trial-form" onSubmit={handleSubmit}>
      <fieldset>
        <legend><span>01</span> About the learner</legend>
        <div className="form-grid form-grid-two">
          <label><span>Guardian or adult learner name</span><input name="guardianName" autoComplete="name" required />{fieldErrors.guardianName && <small className="field-error">{fieldErrors.guardianName}</small>}</label>
          <label><span>Student name <small>Optional</small></span><input name="studentName" autoComplete="off" /></label>
        </div>
        <div className="form-grid form-grid-two">
          <label>
            <span>Student age</span>
            <select name="studentAgeBand" required defaultValue="" onChange={(event) => { setSlotsLoading(true); setSlotsUnavailable(false); setAgeBand(event.target.value as AgeBand); }}>
              <option value="" disabled>Select age band</option>
              <option value="4_7">4–7</option><option value="8_12">8–12</option><option value="13_17">13–17</option><option value="18_plus">18+</option>
            </select>
          </label>
          <label><span>Two-letter country code</span><input name="country" autoComplete="country" required minLength={2} maxLength={2} pattern="[A-Za-z]{2}" placeholder="IN" /></label>
        </div>
        <label><span>Current chess experience <small>Optional</small></span><textarea name="experience" rows={3} placeholder="For example: new to chess, plays online, or has tournament experience" /></label>
        <label className="rating-field"><span>Published rating <small>Optional — leave blank if unsure</small></span><input type="number" name="rating" min="100" max="3500" inputMode="numeric" /></label>
      </fieldset>

      <fieldset>
        <legend><span>02</span> Choose a format and time</legend>
        <div className="segmented-control" aria-label="Preferred trial format">
          <button type="button" className={format === "group" ? "is-selected" : ""} onClick={() => selectFormat("group")}>Small-group assessment</button>
          <button type="button" className={format === "one_to_one" ? "is-selected" : ""} onClick={() => selectFormat("one_to_one")}>One-to-one assessment</button>
        </div>
        <p className="timezone-note">Times shown in {timeZone.replaceAll("_", " ")}.</p>
        {slotsLoading ? (
          <div className="slot-loading" aria-live="polite">Checking available assessment times…</div>
        ) : slots.length ? (
          <div className="slot-grid">
            {slots.map((slot) => (
              <label className="slot-option" key={slot.id}>
                <input type="radio" name="slotId" value={slot.id} required />
                <span><strong>{formatSlot(slot, timeZone)}</strong><small>{slot.format.replaceAll("_", " ")}</small></span>
              </label>
            ))}
          </div>
        ) : (
          <div className="slot-empty">
            <strong>{slotsUnavailable ? "Live availability is temporarily unavailable." : "No published times match this format yet."}</strong>
            <span>You can still send your details and the academy will arrange a suitable time.</span>
          </div>
        )}
      </fieldset>

      <fieldset>
        <legend><span>03</span> Contact and confirm</legend>
        <div className="form-grid form-grid-two">
          <label><span>Email</span><input type="email" name="email" autoComplete="email" required />{fieldErrors.email && <small className="field-error">{fieldErrors.email}</small>}</label>
          <label><span>Phone or WhatsApp <small>Optional</small></span><input type="tel" name="phone" autoComplete="tel" />{fieldErrors.phone && <small className="field-error">{fieldErrors.phone}</small>}</label>
        </div>
        <label className="check-field"><input type="checkbox" name="contactConsent" required /><span>I agree that the academy may contact me to arrange and deliver this trial.</span></label>
        <label className="check-field check-field-muted"><input type="checkbox" name="marketingConsent" /><span>I would also like occasional academy updates. This is optional.</span></label>
      </fieldset>
      {message && <p className="form-message form-message-error" role="alert">{message}</p>}
      <button className="button button-gold button-wide" disabled={state === "submitting" || slotsLoading}>
        {state === "submitting" ? "Securing your request…" : slots.length ? "Confirm trial" : "Request a trial time"}
      </button>
      <p className="form-fineprint">{isDemoMode() ? "Local demo: your enquiry and booking stay in this browser. No messages are sent." : "Your contact and optional marketing choices are recorded with your enquiry."}</p>
    </form>
  );
}
