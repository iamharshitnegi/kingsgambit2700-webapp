import type { Metadata } from "next";
import Link from "next/link";
import PageHero from "../components/public/PageHero";
import PublicShell from "../components/public/PublicShell";
import { getPrograms } from "../lib/public-data";

export const metadata: Metadata = { title: "Online chess coaching programs", description: "Explore small-group, one-to-one, and assessment paths at KingsGambit2700." };

export default async function ProgramsPage() {
  const programs = await getPrograms();
  return <PublicShell><main><PageHero eyebrow="Programs" title="A way to learn for every kind of player." description="Learn together in a small group, focus on your goals with individual coaching, or start with a trial to find your level. We will help you choose the right fit." aside={<div className="hero-note"><span>Made for your learner</span><strong>Level first.<br />Format second.<br />Progress always.</strong></div>} /><section className="program-detail-list">{programs.map((program, index) => <article id={program.slug} className="program-detail" key={program.id}><div className="program-detail-number">0{index + 1}</div><div className="program-detail-main"><span className="eyebrow">{program.formats.join(" · ")}</span><h2>{program.name}</h2><p>{program.summary}</p></div><dl className="program-detail-facts"><div><dt>Suitable for</dt><dd>{program.ageBands.join(", ")}</dd></div>{program.capacity && <div><dt>Class size</dt><dd>Up to {program.capacity} {program.capacity === 1 ? "student" : "students"}</dd></div>}{program.durationMinutes && <div><dt>Session length</dt><dd>{program.durationMinutes} minutes</dd></div>}</dl><Link href="/free-trial" className="button button-outline">Book a free trial</Link></article>)}</section><section className="decision-band"><div><span className="eyebrow eyebrow-light">Not sure which path fits?</span><h2>Let’s find the right starting point together.</h2></div><Link href="/free-trial" className="button button-cream">Choose a trial time</Link></section></main></PublicShell>;
}
