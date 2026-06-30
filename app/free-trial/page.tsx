import Image from 'next/image';
import StatsSection from '../components/StatsSection';
import CourseLevels from '../components/CourseLevels';
import LeadForm from '../components/LeadForm';
import MinimalFooter from '../components/MinimalFooter';
import Link from 'next/link';

export const metadata = {
  title: 'Book Your Free Chess Trial Class | KingsGambit2700',
  description: 'Join KingsGambit2700 for a free online chess trial class. Learn from FIDE-certified coaches and improve your strategy.',
};

export default function FreeTrialLanding() {
  return (
    <div className="bg-brand-cream min-h-screen text-brand-charcoal flex flex-col">
      {/* Minimal Header */}
      <header className="py-6 px-6 md:px-12 border-b border-brand-border relative z-50">
        <div className="max-w-7xl mx-auto flex justify-between items-center">
          <Link href="/" className="flex items-center gap-3 font-serif text-xl md:text-2xl text-brand-charcoal tracking-[0.3px] font-bold">
            <img src="/logo.png" alt="KingsGambit 2700" className="h-8 md:h-10 w-auto" />
            KingsGambit <span className="text-brand-gold italic">2700</span>
          </Link>
          <div className="text-xs font-bold text-stone-500 uppercase tracking-widest hidden sm:block">
            Professional Chess Coaching
          </div>
        </div>
      </header>

      {/* Hero Section with Form */}
      <section className="relative pt-16 pb-24 lg:pt-28 lg:pb-32 flex-grow">
        <div className="max-w-7xl mx-auto px-6 lg:px-8">
          <div className="grid lg:grid-cols-2 gap-16 lg:gap-24 items-start">
            {/* Value Proposition */}
            <div>
              <div className="text-brand-gold text-xs uppercase tracking-[0.2em] font-bold mb-6">
                Limited Spots Available
              </div>
              <h1 className="text-5xl md:text-6xl font-serif font-medium leading-[1.1] mb-8 text-brand-charcoal">
                Unlock your child's true <span className="text-brand-gold italic">potential</span>
              </h1>
              <div className="w-16 h-[1px] bg-brand-border mb-8"></div>
              <p className="text-lg md:text-xl text-stone-600 mb-10 leading-relaxed">
                Experience world-class online coaching from FIDE-certified Grandmasters. Build critical thinking, focus, and strategic skills.
              </p>
              
              <ul className="space-y-5">
                <li className="flex items-start gap-4">
                  <span className="text-brand-goldLight text-sm mt-1"><i className="fa-solid fa-check"></i></span>
                  <span className="text-brand-charcoal text-base md:text-lg">Structured FIDE-rated curriculum</span>
                </li>
                <li className="flex items-start gap-4">
                  <span className="text-brand-goldLight text-sm mt-1"><i className="fa-solid fa-check"></i></span>
                  <span className="text-brand-charcoal text-base md:text-lg">Small batch sizes for personal attention</span>
                </li>
                <li className="flex items-start gap-4">
                  <span className="text-brand-goldLight text-sm mt-1"><i className="fa-solid fa-check"></i></span>
                  <span className="text-brand-charcoal text-base md:text-lg">100% online, flexible timings</span>
                </li>
              </ul>
            </div>

            {/* Form Container */}
            <div className="bg-white rounded-md shadow-sm border border-brand-border p-10 md:p-12 relative">
              <h2 className="text-3xl font-serif font-medium text-brand-charcoal mb-4">Book your trial</h2>
              <p className="text-stone-600 mb-8 text-base">Fill out the form below and we'll schedule a time that works for you.</p>
              
              <LeadForm />
              
              <p className="text-xs text-stone-400 text-center mt-6 uppercase tracking-wider font-medium">
                We respect your privacy. No spam, ever.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* Social Proof / Stats */}
      <section className="bg-brand-cream border-t border-brand-border">
        <StatsSection />
      </section>

      {/* Testimonial Highlight */}
      <section className="py-24 bg-brand-cream border-t border-brand-border">
        <div className="max-w-4xl mx-auto px-6 text-center">
          <div className="flex justify-center gap-1.5 text-brand-gold text-xs mb-8">
            <i className="fa-solid fa-star"></i>
            <i className="fa-solid fa-star"></i>
            <i className="fa-solid fa-star"></i>
            <i className="fa-solid fa-star"></i>
            <i className="fa-solid fa-star"></i>
          </div>
          <h3 className="text-3xl md:text-4xl font-serif font-medium text-brand-charcoal mb-10 italic leading-relaxed">
            "My son's focus has improved tremendously in just 3 months. The coaches are simply exceptional."
          </h3>
          <p className="font-bold text-brand-charcoal text-base">Priya Sharma</p>
          <p className="text-stone-500 uppercase tracking-widest text-xs mt-2">Parent from Dubai</p>
        </div>
      </section>

      <MinimalFooter />
    </div>
  );
}
