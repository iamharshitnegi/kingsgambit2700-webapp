"use client";

import { useState } from "react";
import { getFieldErrors } from "@/app/lib/api/client";
import { createLead } from "@/app/lib/api/typed-client";
import type { AgeBand, LeadInput, TrialFormat } from "@/app/lib/api/types";

type FormState = "idle" | "submitting" | "success" | "error";

export default function InquiryForm() {
  const [state, setState] = useState<FormState>("idle");
  const [message, setMessage] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Kolkata";
    const email = String(data.get("email") || "").trim();
    const phone = String(data.get("phone") || "").trim();

    if (!email && !phone) {
      setFieldErrors({ email: "Add an email address or phone number." });
      return;
    }

    const payload: LeadInput = {
      guardianName: String(data.get("guardianName") || "").trim(),
      email: email || undefined,
      phone: phone || undefined,
      studentAgeBand: String(data.get("studentAgeBand") || "") as AgeBand,
      experience: String(data.get("experience") || "").trim() || undefined,
      country: String(data.get("country") || "").trim().toUpperCase(),
      timeZone,
      preferredFormat: (String(data.get("preferredFormat") || "") || undefined) as TrialFormat | undefined,
      preferredTimes: [String(data.get("preferredTimes") || "").trim()].filter(Boolean),
      contactConsent: data.get("contactConsent") === "on",
      marketingConsent: data.get("marketingConsent") === "on",
      source: "contact",
      landingPage: window.location.href,
      referrer: document.referrer || undefined,
    };

    setState("submitting");
    setMessage("");
    setFieldErrors({});
    try {
      await createLead(payload);
      form.reset();
      setState("success");
      setMessage("Your enquiry is safely recorded. The academy will use your chosen contact details to follow up.");
    } catch (error) {
      setFieldErrors(getFieldErrors(error));
      setState("error");
      setMessage(error instanceof Error ? error.message : "We could not save your enquiry.");
    }
  }

  if (state === "success") {
    return (
      <div className="form-success" role="status">
        <span className="form-success-mark">✓</span>
        <h2>Thank you — we have your enquiry.</h2>
        <p>{message}</p>
        <button className="text-link" onClick={() => setState("idle")}>Send another enquiry</button>
      </div>
    );
  }

  return (
    <form className="academy-form" onSubmit={handleSubmit}>
      <div className="form-grid form-grid-two">
        <label>
          <span>Your name</span>
          <input name="guardianName" autoComplete="name" required />
          {fieldErrors.guardianName && <small className="field-error">{fieldErrors.guardianName}</small>}
        </label>
        <label>
          <span>Student age</span>
          <select name="studentAgeBand" required defaultValue="">
            <option value="" disabled>Select an age band</option>
            <option value="4_7">4–7</option>
            <option value="8_12">8–12</option>
            <option value="13_17">13–17</option>
            <option value="18_plus">18+</option>
          </select>
        </label>
      </div>
      <div className="form-grid form-grid-two">
        <label>
          <span>Email</span>
          <input type="email" name="email" autoComplete="email" />
          {fieldErrors.email && <small className="field-error">{fieldErrors.email}</small>}
        </label>
        <label>
          <span>Phone or WhatsApp</span>
          <input type="tel" name="phone" autoComplete="tel" />
          {fieldErrors.phone && <small className="field-error">{fieldErrors.phone}</small>}
        </label>
      </div>
      <div className="form-grid form-grid-two">
        <label>
          <span>Two-letter country code</span>
          <input name="country" autoComplete="country" required minLength={2} maxLength={2} pattern="[A-Za-z]{2}" placeholder="IN" />
        </label>
        <label>
          <span>Preferred format</span>
          <select name="preferredFormat" defaultValue="">
            <option value="">Help me choose</option>
            <option value="group">Small group</option>
            <option value="one_to_one">One to one</option>
          </select>
        </label>
      </div>
      <label>
        <span>Current chess experience</span>
        <textarea name="experience" rows={3} placeholder="What can the student do confidently today?" />
      </label>
      <label>
        <span>Convenient times</span>
        <input name="preferredTimes" placeholder="For example: weekday evenings" />
      </label>
      <label className="check-field">
        <input type="checkbox" name="contactConsent" required />
        <span>I agree that the academy may contact me about this enquiry.</span>
      </label>
      <label className="check-field check-field-muted">
        <input type="checkbox" name="marketingConsent" />
        <span>I would also like occasional academy updates. This is optional.</span>
      </label>
      {message && <p className="form-message form-message-error" role="alert">{message}</p>}
      <button className="button button-dark button-wide" disabled={state === "submitting"}>
        {state === "submitting" ? "Saving your enquiry…" : "Send enquiry"}
      </button>
    </form>
  );
}
