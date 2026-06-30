import Navbar from './components/Navbar';
import Footer from './components/Footer';
import StatsSection from './components/StatsSection';
import CourseLevels from './components/CourseLevels';
import { WHATSAPP_NUMBER } from './constants';
import Link from 'next/link';

export default function Home() {
  return (
    <>
      <Navbar />

      {/* Hero Section */}
      <section className="pt-32 pb-20 md:pt-40 md:pb-24 px-6 lg:px-8 text-center bg-brand-cream">
        <div className="max-w-4xl mx-auto">
          <div className="text-brand-gold text-xs md:text-sm uppercase tracking-[0.2em] font-bold mb-6">
            Professional chess coaching
          </div>
          <h1 className="text-5xl md:text-6xl lg:text-7xl font-serif font-medium text-brand-charcoal leading-[1.1] mb-8">
            From beginner to <span className="text-brand-gold italic">grandmaster</span>
          </h1>
          <p className="text-stone-600 text-lg md:text-xl max-w-2xl mx-auto mb-12 leading-relaxed">
            Building strategy, focus and confidence, one game at a time.
          </p>
          <div className="flex flex-col sm:flex-row justify-center gap-4">
            <Link
              href={`https://wa.me/${WHATSAPP_NUMBER}?text=I'm%20interested%20in%20a%20trial%20class`}
              target="_blank"
              className="bg-brand-charcoal text-brand-cream text-[13px] font-medium px-8 py-3.5 rounded-md hover:bg-stone-800 transition shadow-sm"
            >
              Get a free trial
            </Link>
            <Link
              href="#courses"
              className="border border-brand-charcoal text-brand-charcoal text-[13px] font-medium px-8 py-3.5 rounded-md hover:bg-black/5 transition"
            >
              Explore programs
            </Link>
          </div>
        </div>
      </section>

      {/* Stats Section */}
      <StatsSection />

      {/* Vision Section */}
      <section id="vision" className="py-20 md:py-28 bg-brand-cream">
        <div className="max-w-7xl mx-auto px-6 lg:px-8">
          <div className="grid lg:grid-cols-5 gap-12 lg:gap-20 items-center">
            <div className="lg:col-span-3">
              <div className="text-brand-gold text-xs uppercase tracking-[0.2em] font-bold mb-6">Our Vision</div>
              <h2 className="text-4xl lg:text-5xl font-serif text-brand-charcoal mb-8 leading-tight">More than just a game</h2>
              <div className="w-16 h-[1px] bg-brand-border mb-8"></div>
              <p className="text-stone-600 text-base md:text-lg leading-relaxed mb-6">
                At KingsGambit2700, we believe chess is a tool for life. Our vision is to cultivate critical thinking, patience, and sportsmanship in every student.
              </p>
              <p className="text-stone-600 text-base md:text-lg leading-relaxed mb-8">
                Whether you are looking to compete at the national level or simply want to improve concentration, our curriculum is designed to adapt to your goals.
              </p>
              <ul className="space-y-5">
                <li className="flex items-start gap-4">
                  <span className="text-brand-goldLight text-sm mt-1"><i className="fa-solid fa-check"></i></span>
                  <span className="text-brand-charcoal text-base md:text-lg">Structured FIDE-rated curriculum</span>
                </li>
                <li className="flex items-start gap-4">
                  <span className="text-brand-goldLight text-sm mt-1"><i className="fa-solid fa-check"></i></span>
                  <span className="text-brand-charcoal text-base md:text-lg">Small batch sizes (Max 6 students)</span>
                </li>
                <li className="flex items-start gap-4">
                  <span className="text-brand-goldLight text-sm mt-1"><i className="fa-solid fa-check"></i></span>
                  <span className="text-brand-charcoal text-base md:text-lg">Regular internal tournaments</span>
                </li>
              </ul>
            </div>
            <div className="relative lg:col-span-2 mt-8 lg:mt-0">
              <div className="aspect-[4/5] overflow-hidden rounded-md shadow-lg">
                <img src="https://images.unsplash.com/photo-1580541832626-2a7131ee809f?ixlib=rb-1.2.1&auto=format&fit=crop&w=1000&q=80" alt="Chess Coaching" className="w-full h-full object-cover sepia-[0.2] grayscale-[0.2] hover:scale-105 transition duration-700" />
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Courses Section */}
      <section id="courses" className="py-20 md:py-28 bg-brand-cream border-t border-brand-border">
        <div className="max-w-7xl mx-auto px-6 lg:px-8">
          <div className="text-center mb-16">
            <h2 className="text-4xl lg:text-5xl font-serif text-brand-charcoal">Training levels</h2>
            <p className="text-stone-600 text-base md:text-lg mt-6 max-w-md mx-auto">Structured paths for every age and skill level</p>
          </div>

          <CourseLevels />
        </div>
      </section>

      {/* Video Demo Section */}
      <section className="py-20 md:py-28 bg-stone-100/50 border-t border-brand-border">
        <div className="max-w-5xl mx-auto px-6 lg:px-8 text-center">
          <div className="text-brand-gold text-xs uppercase tracking-[0.2em] font-bold mb-6">Methodology</div>
          <h2 className="text-4xl lg:text-5xl font-serif text-brand-charcoal mb-12">See our classes in action</h2>

          <div className="aspect-video bg-stone-200 rounded-md overflow-hidden relative group">
            <img
              src="https://images.unsplash.com/photo-1586165368502-1bad197a6461?ixlib=rb-1.2.1&auto=format&fit=crop&w=1950&q=80"
              alt="Chess Class Demo"
              className="w-full h-full object-cover mix-blend-multiply opacity-80"
            />
            <div className="absolute inset-0 flex items-center justify-center">
              <button className="w-16 h-16 bg-brand-cream text-brand-charcoal rounded-full flex items-center justify-center text-xl hover:scale-105 transition shadow-sm">
                <i className="fa-solid fa-play ml-1"></i>
              </button>
            </div>
          </div>
        </div>
      </section>

      {/* Testimonials / Review Section */}
      <section className="py-20 md:py-28 bg-brand-cream border-t border-brand-border">
        <div className="max-w-7xl mx-auto px-6 lg:px-8">
          <div className="text-center mb-16">
            <h2 className="text-4xl lg:text-5xl font-serif text-brand-charcoal mb-6">What parents say</h2>
            <div className="flex justify-center gap-1.5 text-brand-gold text-xs">
              <i className="fa-solid fa-star"></i>
              <i className="fa-solid fa-star"></i>
              <i className="fa-solid fa-star"></i>
              <i className="fa-solid fa-star"></i>
              <i className="fa-solid fa-star"></i>
            </div>
          </div>

          <div className="grid md:grid-cols-3 gap-8 max-w-5xl mx-auto">
            {[
              { name: "Priya Sharma", city: "Dubai", text: "The coaches at KingsGambit2700 are exceptional. My son's focus has improved tremendously in just 3 months." },
              { name: "Rahul Verma", city: "Bengaluru", text: "Best online chess academy. The small batch size ensures personal attention to every child." },
              { name: "Sarah J.", city: "London", text: "Flexible timings and world-class curriculum. Highly recommended for young aspiring players." }
            ].map((review, i) => (
              <div key={i} className="p-8 md:p-10 border border-brand-border bg-white/50 rounded-md">
                <p className="text-stone-700 mb-8 font-serif italic text-base md:text-lg leading-relaxed">"{review.text}"</p>
                <div>
                  <p className="text-sm font-bold text-brand-charcoal">{review.name}</p>
                  <p className="text-xs text-stone-500 uppercase tracking-[0.1em] mt-1.5">{review.city}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Coaches Section */}
      <section id="coaches" className="py-20 md:py-28 bg-brand-cream border-t border-brand-border">
        <div className="max-w-3xl mx-auto px-6 lg:px-8 text-center">
          <h2 className="text-4xl lg:text-5xl font-serif text-brand-charcoal mb-8">Our expert coaches</h2>
          <p className="text-base md:text-lg text-stone-600 leading-relaxed mb-10">
            We have highly experienced <strong className="text-brand-charcoal font-bold">FIDE-certified coaches</strong> who cater to the unique needs of our students. From building strong foundations to mastering complex strategies.
          </p>
          <div className="flex flex-wrap justify-center gap-4">
            <a href="https://ratings.fide.com" target="_blank" rel="noopener noreferrer" className="text-[12px] text-brand-charcoal border border-brand-border px-6 py-3 rounded-md hover:bg-black/5 transition flex items-center gap-2">
              <i className="fa-solid fa-award text-brand-gold"></i> View FIDE Coach Profile
            </a>
            <a href="https://ratings.fide.com" target="_blank" rel="noopener noreferrer" className="text-[12px] text-brand-charcoal border border-brand-border px-6 py-3 rounded-md hover:bg-black/5 transition flex items-center gap-2">
              <i className="fa-solid fa-award text-brand-gold"></i> Certified Mental Coach
            </a>
          </div>
        </div>
      </section>

      <Footer />
    </>
  );
}