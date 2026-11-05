import type { Metadata } from "next";
import Link from "next/link";
import PageHero from "../components/public/PageHero";
import PublicShell from "../components/public/PublicShell";
import { getCurriculum } from "../lib/public-data";

export const metadata: Metadata = { title: "Chess curriculum and levels", description: "See the KingsGambit2700 public chess learning framework and level outcomes." };

export default async function CurriculumPage() {
  const curriculum = await getCurriculum();
  return <PublicShell><main><PageHero eyebrow="Our curriculum" title="A learning path with visible outcomes." description="The framework moves from board confidence to calculation, planning, reflective analysis, and competitive preparation. Placement is confirmed through a live assessment." aside={<div className="notation-card" aria-hidden="true"><span>1.</span> notice <em>…</em><span>2.</span> calculate <em>…</em><span>3.</span> decide</div>} /><section className="curriculum-page-list">{curriculum.tracks.map((track, trackIndex) => <article className="curriculum-track" id={track.slug} key={track.slug}><header><span className="track-number">0{trackIndex + 1}</span><div><span className="eyebrow">Learning track</span><h2>{track.name}</h2><p>{track.summary}</p></div></header><div className="level-grid">{track.levels.map((level, levelIndex) => <section key={level.slug}><div className="level-kicker">{trackIndex + 1}.{levelIndex + 1}</div><h3>{level.name}</h3><p>{level.summary}</p>{(level.ageGuidance || level.ratingGuidance) && <div className="guidance">{[level.ageGuidance, level.ratingGuidance].filter(Boolean).join(" · ")}</div>}<h4>What you will learn</h4><ul>{level.outcomes.map((outcome) => <li key={outcome}>{outcome}</li>)}</ul></section>)}</div></article>)}</section><section className="assessment-callout"><div><span className="eyebrow">Placement</span><h2>A starting point that feels right.</h2><p>Your trial helps us understand how you approach the board, which skills feel comfortable, and what to work on next.</p></div><Link href="/free-trial" className="button button-dark">Book a free trial</Link></section></main></PublicShell>;
}
