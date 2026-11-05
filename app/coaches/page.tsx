/* Dynamic coach photo origins and source dimensions come from the verified public API. */
/* eslint-disable @next/next/no-img-element */
import type { Metadata } from "next";
import Link from "next/link";
import PageHero from "../components/public/PageHero";
import PublicShell from "../components/public/PublicShell";
import { getCoaches } from "../lib/public-data";

export const metadata: Metadata = { title: "Chess coaches", description: "View public, verified KingsGambit2700 coach profiles." };

export default async function CoachesPage() {
  const coaches = await getCoaches();
  return <PublicShell><main><PageHero eyebrow="Coaching team" title="Meet the people in your corner." description="Patient explanations, thoughtful feedback, and a shared love of the game. We match students with a coach who understands their level, goals, and learning style." aside={<div className="hero-note"><span>A personal approach</span><strong>Room for questions.<br />Time to think.<br />Guidance to grow.</strong></div>} /><section className="coach-page-grid">{coaches.length ? coaches.map((coach) => <article className="coach-card" id={coach.id} key={coach.id}><div className="coach-portrait">{coach.photoUrl ? <img src={coach.photoUrl} alt={`Portrait of ${coach.displayName}`} /> : <span>{coach.displayName.slice(0, 1)}</span>}</div><div className="coach-card-content"><span className="eyebrow">{"Academy coach"}</span><h2>{coach.displayName}</h2><p>{coach.bio}</p><dl>{coach.title && <div><dt>Title</dt><dd>{coach.title}</dd></div>}{coach.rating && <div><dt>Rating</dt><dd>{coach.rating}{coach.ratingSource ? ` · ${coach.ratingSource}` : ""}</dd></div>}{coach.languages.length > 0 && <div><dt>Languages</dt><dd>{coach.languages.join(", ")}</dd></div>}</dl></div></article>) : <div className="empty-editorial"><span className="eyebrow">Find your coach</span><h2>A great fit starts with a conversation.</h2><p>Our team can help match your learner with a coach for their level, preferred language, and class format. Get in touch to meet the team and discuss availability.</p><Link href="/contact" className="button button-dark">Ask about coach matching</Link></div>}</section><section className="coach-principles"><div><span>01</span><h3>The right fit</h3><p>The right coach understands the learner’s level and communication needs.</p></div><div><span>02</span><h3>Clear feedback</h3><p>Understand what is going well and what to practise next.</p></div><div><span>03</span><h3>Parents included</h3><p>Stay connected to the learning journey through your parent account.</p></div></section></main></PublicShell>;
}
