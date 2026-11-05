import type { Metadata } from "next";
import Link from "next/link";
import { CITY_DATA } from "../constants/cities";
import { getPrograms } from "../lib/public-data";
import PublicShell from "./public/PublicShell";

export function getLocationMetadata(cityId: string): Metadata {
  const city = CITY_DATA[cityId];
  if (!city) return {};
  return {
    title: city.headline,
    description: city.description,
    alternates: { canonical: `/online-chess-classes-${cityId}` },
  };
}

export default async function LocationPageTemplate({ cityId }: { cityId: string }) {
  const city = CITY_DATA[cityId];
  if (!city) return null;
  const programs = await getPrograms();

  return (
    <PublicShell>
      <main>
        <section className="location-hero">
          <div><span className="eyebrow eyebrow-light">Live online · {city.timeZoneLabel}</span><h1>{city.headline}</h1><p>{city.subheadline}</p><div><Link href="/free-trial" className="button button-cream">View trial availability</Link><Link href="/curriculum" className="text-link text-link-light">Explore levels</Link></div></div>
          <div className="timezone-dial" aria-hidden="true"><span>LOCAL</span><strong>{city.name}</strong><small>{city.timeZone}</small></div>
        </section>
        <section className="location-intro"><div><span className="eyebrow">Designed for online learning</span><h2>Location changes the clock, not the learning standard.</h2></div><div><p>{city.description}</p><p>Times shown on the booking page are converted in the browser. The final schedule and availability remain controlled by the academy service.</p></div></section>
        <section className="location-programs"><div className="section-heading"><span className="eyebrow">Available pathways</span><h2>Begin with the right format and level.</h2></div><div className="program-grid">{programs.slice(0, 3).map((program, index) => <article className="program-card" key={program.id}><span className="card-index">0{index + 1}</span><div className="program-format">{program.formats.join(" · ")}</div><h3>{program.name}</h3><p>{program.summary}</p><Link href="/free-trial">Start with an assessment <span>→</span></Link></article>)}</div></section>
        <section className="assessment-callout"><div><span className="eyebrow">Local-time booking</span><h2>Choose a published time without doing the time-zone maths.</h2><p>The booking experience detects your IANA time zone and labels each assessment time locally before you confirm.</p></div><Link href="/free-trial" className="button button-dark">Book from {city.name}</Link></section>
      </main>
    </PublicShell>
  );
}
