import Link from "next/link";
import Image from "next/image";
import PublicShell from "./components/public/PublicShell";
import { getCoaches, getCurriculum, getPrograms } from "./lib/public-data";

const opening = [
  "♜", "♞", "♝", "♛", "♚", "♝", "♞", "♜",
  "♟", "♟", "♟", "♟", "", "♟", "♟", "♟",
  "", "", "", "", "", "", "", "",
  "", "", "", "", "♟", "", "", "",
  "", "", "", "", "♙", "♙", "", "",
  "", "", "", "", "", "", "", "",
  "♙", "♙", "♙", "♙", "", "", "♙", "♙",
  "♖", "♘", "♗", "♕", "♔", "♗", "♘", "♖",
];

const questions = [
  {
    question: "Does my child need to know how to play?",
    answer: "No. Beginners can start with how the pieces move, while experienced players begin with a level assessment. The first session helps us choose a useful starting point for your child.",
  },
  {
    question: "How do online classes work?",
    answer: "Students learn live with a coach, discuss positions on a shared chessboard, and practise what they have learned. A laptop or desktop with a reliable internet connection is best for participating in class.",
  },
  {
    question: "Should we choose a small group or one-to-one coaching?",
    answer: "Small groups give students room to discuss ideas and learn together. One-to-one sessions provide more individual attention and a personalised practice plan. We can help you choose after the trial.",
  },
  {
    question: "How will I know my child is progressing?",
    answer: "Your parent account brings together class attendance, assignments, coach feedback, and published progress reports. The curriculum also explains the skills your child is working towards at each level.",
  },
];

export default async function HomePage() {
  const [programs, curriculum, coaches] = await Promise.all([
    getPrograms(),
    getCurriculum(),
    getCoaches(),
  ]);

  return (
    <PublicShell>
      <main className="academy-home">
        <section className="academy-hero" aria-labelledby="home-heading">
          <div className="academy-hero-copy">
            <span className="eyebrow">Professional chess coaching · live online</span>
            <h1 id="home-heading">From beginner <br />to <em>grandmaster</em></h1>
            <p>Building strategy, focus and confidence,<br className="hero-line-break" /> one game at a time.</p>
            <div className="hero-actions">
              <Link href="/free-trial" className="button button-primary">Book a free trial <span aria-hidden="true">↗</span></Link>
              <Link href="#courses" className="academy-hero-explore">Explore programs <span aria-hidden="true">→</span></Link>
            </div>
            <span className="hero-footnote">For first moves, big ambitions, and everything in between.</span>
            <div className="academy-hero-community" aria-hidden="true"><span><i>♙</i><i>♘</i><i>♗</i></span><p>Different starting points.<br /><strong>One shared love of the game.</strong></p></div>
          </div>
          <div className="academy-hero-visual">
            <div className="academy-art-label" aria-hidden="true"><span>✧</span><p>The next move<br />is theirs.</p></div>
            <Image className="academy-hero-art" src="/brand/chess-sculpture-brown.png" width={1199} height={1312} sizes="(max-width: 620px) 100vw, 50vw" alt="A sculptural warm brown knight and a sand-gold pawn on ivory and brown chessboard tiles" preload />
            <div className="academy-art-note"><span aria-hidden="true">↗</span><div><strong>More than a good move.</strong><p>A more confident mind.</p></div><b aria-hidden="true">↗</b></div>
            <span className="academy-art-caption">Play is a seriously good teacher.</span>
          </div>
        </section>

        <div className="academy-values" aria-label="The academy at a glance">
          <span><b aria-hidden="true">♙</b> Small groups. Personal attention.</span>
          <span><b aria-hidden="true">♘</b> A clear path for every level.</span>
          <span><b aria-hidden="true">♔</b> Parents stay in the picture.</span>
        </div>

        <section id="vision" className="academy-vision editorial-section" aria-labelledby="vision-heading">
          <div className="academy-vision-copy">
            <span className="eyebrow">Our vision</span>
            <h2 id="vision-heading">More than<br />just a game.</h2>
            <p>Every position is a chance to pause, think, and make a choice. We help students turn those small moments into lasting habits of patience, curiosity, and confidence.</p>
            <p>Whether your child is learning the rules or preparing for a tournament, our coaching meets them where they are and gives them a clear next step.</p>
            <ul className="academy-checklist">
              <li>Live lessons with a coach who knows the learner</li>
              <li>Structured levels, from foundations to competition</li>
              <li>Purposeful practice and feedback you can follow</li>
            </ul>
            <Link href="/curriculum" className="arrow-link">Discover our approach <span aria-hidden="true">→</span></Link>
          </div>
          <figure className="academy-opening">
            <div className="opening-label"><span>The art of the first move</span><span aria-hidden="true">KG / 2700</span></div>
            <div className="opening-board" role="img" aria-label="The King's Gambit opening, after 1. e4 e5 2. f4. White offers a pawn to take control of the centre.">
              {opening.map((piece, index) => (
                <span className={`opening-square ${(Math.floor(index / 8) + index) % 2 ? "square-dark" : "square-light"}${index === 37 ? " square-highlight" : ""}`} key={index} aria-hidden="true">
                  <b className={index >= 32 ? "white-piece" : "black-piece"}>{piece}</b>
                </span>
              ))}
            </div>
            <figcaption><div><span>The King’s Gambit</span><strong>A little courage. A clear plan.</strong></div><span className="opening-moves">1. e4 e5<br />2. f4</span></figcaption>
          </figure>
        </section>

        <section id="courses" className="card-section academy-programs" aria-labelledby="programs-heading">
          <span id="programs" className="anchor-offset" aria-hidden="true" />
          <div className="section-heading section-heading-row">
            <div><span className="eyebrow">Find your path</span><h2 id="programs-heading">Different learners.<br />The right kind of guidance.</h2></div>
            <Link href="/programs" className="arrow-link">All programs <span aria-hidden="true">→</span></Link>
          </div>
          <div className="program-grid">
            {programs.slice(0, 3).map((program, index) => (
              <article className="program-card" key={program.id}>
                <div className="program-card-top"><span className="card-index">0{index + 1}</span><span aria-hidden="true">{["♙", "♘", "♕"][index]}</span></div>
                <div className="program-format">{program.formats.join(" · ")}</div>
                <h3>{program.name}</h3>
                <p>{program.summary}</p>
                <ul>{program.ageBands.map((band) => <li key={band}>{band}</li>)}</ul>
                <Link href={`/programs#${program.slug}`}>Explore {program.name.toLowerCase()} <span aria-hidden="true">↗</span></Link>
              </article>
            ))}
          </div>
        </section>

        <section id="curriculum" className="academy-curriculum" aria-labelledby="curriculum-heading">
          <div className="academy-curriculum-inner">
            <div className="academy-curriculum-copy">
              <span className="eyebrow eyebrow-light">Training levels</span>
              <h2 id="curriculum-heading">One thoughtful<br />move at a time.</h2>
              <p>Strong foundations. Sharper decisions. Confident play. Each stage builds on the last, with clear skills to learn and space to grow.</p>
              <Link href="/curriculum" className="button button-cream">Explore the curriculum <span aria-hidden="true">↗</span></Link>
            </div>
            <div className="academy-tracks">
              {curriculum.tracks.map((track, index) => (
                <Link href={`/curriculum#${track.slug}`} className="academy-track" key={track.slug}>
                  <span className="academy-track-number">0{index + 1}</span>
                  <div><h3>{track.name}</h3><p>{track.summary}</p></div>
                  <span className="academy-track-arrow" aria-hidden="true">↗</span>
                </Link>
              ))}
            </div>
          </div>
        </section>

        <section className="card-section academy-process" aria-labelledby="process-heading">
          <div className="section-heading"><span className="eyebrow">Your first three moves</span><h2 id="process-heading">Getting started is simple.</h2></div>
          <ol className="academy-steps">
            <li><span>01</span><h3>Book a free trial</h3><p>Tell us a little about the learner and choose a convenient assessment time.</p></li>
            <li><span>02</span><h3>Meet your coach</h3><p>Play, ask questions, and let us get to know how your child thinks at the board.</p></li>
            <li><span>03</span><h3>Find your starting point</h3><p>Get a recommendation for the right level and class format before you enrol.</p></li>
          </ol>
        </section>

        <section id="coaches" className="editorial-section academy-coaches" aria-labelledby="coaches-heading">
          <div className="section-heading section-heading-row"><div><span className="eyebrow">The people behind the progress</span><h2 id="coaches-heading">Good teaching.<br />A personal connection.</h2></div><Link href="/coaches" className="arrow-link">Meet the coaches <span aria-hidden="true">→</span></Link></div>
          {coaches.length ? (
            <div className="coach-mini-grid">{coaches.slice(0, 3).map((coach) => <article key={coach.id}><div className="coach-monogram" aria-hidden="true">{coach.displayName.slice(0, 1)}</div><h3>{coach.displayName}</h3><p>{[coach.title, coach.languages.join(" · ")].filter(Boolean).join(" · ") || "Chess coach"}</p><Link href={`/coaches#${coach.id}`} className="arrow-link">View profile <span aria-hidden="true">↗</span></Link></article>)}</div>
          ) : (
            <div className="academy-coach-note"><p>A coach who listens, explains clearly, and makes room for questions can change how a student feels about learning. We match learners by level, goals, and language.</p><Link href="/free-trial" className="button button-outline">Meet a coach at your trial</Link></div>
          )}
        </section>

        <section className="academy-faq editorial-section" aria-labelledby="faq-heading">
          <div className="section-heading"><span className="eyebrow">A few helpful answers</span><h2 id="faq-heading">Before your<br />first move.</h2><Link href="/contact" className="arrow-link">Talk to us <span aria-hidden="true">→</span></Link></div>
          <div className="academy-faq-list">{questions.map((item) => <details key={item.question}><summary>{item.question}<span aria-hidden="true">+</span></summary><p>{item.answer}</p></details>)}</div>
        </section>
      </main>
    </PublicShell>
  );
}
