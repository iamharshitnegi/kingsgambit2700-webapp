import type { Metadata } from "next";
import InquiryForm from "../components/forms/InquiryForm";
import PageHero from "../components/public/PageHero";
import PublicShell from "../components/public/PublicShell";
import { getOrganization } from "../lib/public-data";

export const metadata: Metadata = { title: "Contact", description: "Contact KingsGambit2700 about online chess coaching." };

export default async function ContactPage() {
  const organization = await getOrganization();
  return <PublicShell><main><PageHero eyebrow="Contact" title="Let’s talk about your next move." description="Have a question about classes, your child’s level, or finding a time that works? Send us a message and our team will help." aside={<div className="contact-direct"><span>Direct contact</span><a href={`mailto:${organization.contactEmail}`}>{organization.contactEmail}</a><a href={`https://wa.me/${organization.whatsappNumber.replace(/\D/g, "")}`} target="_blank" rel="noreferrer">WhatsApp {organization.whatsappDisplay}</a></div>} /><section className="contact-form-section"><div><span className="eyebrow">Enquiry details</span><h2>How can we help?</h2><p>For a specific assessment time, use the trial booking page instead.</p></div><InquiryForm /></section></main></PublicShell>;
}
