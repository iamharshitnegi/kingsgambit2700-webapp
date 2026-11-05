import type { Metadata } from "next";
import TrialBookingForm from "../components/forms/TrialBookingForm";
import PublicShell from "../components/public/PublicShell";

export const metadata: Metadata = { title: "Book a chess trial and assessment", description: "Share the learner’s goals, see published trial availability, and book a live KingsGambit2700 assessment." };

export default function FreeTrialPage() {
  return <PublicShell><main className="trial-page"><section className="trial-intro"><div><span className="eyebrow eyebrow-light">Trial and assessment</span><h1>Your first move starts here.</h1><p>Meet a coach, explore a few positions, and get a feel for learning with us. We will help you find the right level and class format.</p></div><ol><li><span>01</span>Tell us what the learner can do today.</li><li><span>02</span>Meet live for a practical chess assessment.</li><li><span>03</span>Review a level and format recommendation.</li></ol></section><section className="trial-form-wrap"><div className="trial-form-heading"><span className="eyebrow">Book your assessment</span><h2>Let’s get to know the learner.</h2><p>Share a few details, then choose a trial time. A parent or guardian should book for a child.</p></div><TrialBookingForm /></section></main></PublicShell>;
}
