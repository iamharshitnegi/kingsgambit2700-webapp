import { Metadata } from 'next';
import Navbar from './Navbar';
import Footer from './Footer';
import CourseLevels from './CourseLevels';
import { CITY_DATA } from '../constants/cities';
import Link from 'next/link';

interface Props {
  cityId: string;
}

export function getLocationMetadata(cityId: string): Metadata {
  const city = CITY_DATA[cityId];
  if (!city) return {};

  return {
    title: city.headline,
    description: city.description,
    alternates: {
      canonical: `https://kingsgambit2700.in/online-chess-classes-${cityId}`,
    },
  };
}

export default function LocationPageTemplate({ cityId }: Props) {
  const city = CITY_DATA[cityId];

  if (!city) return null;

  return (
    <>
      <Navbar />
      <main className="pt-20">
        {/* Localized Hero Section */}
        <section className="bg-brand-dark text-white py-20 px-6 lg:px-8 text-center relative overflow-hidden">
          <div className="absolute inset-0 opacity-10 pointer-events-none">
            <img src="https://images.unsplash.com/photo-1529699211952-734e80c4d42b?ixlib=rb-1.2.1&auto=format&fit=crop&w=1950&q=80" alt="Chess Background" className="w-full h-full object-cover" />
          </div>
          <div className="max-w-4xl mx-auto relative z-10">
            <h1 className="text-4xl md:text-6xl font-serif font-bold mb-6">
              {city.headline}
            </h1>
            <p className="text-xl text-gray-300 mb-8">
              {city.subheadline}
            </p>
            <div className="flex flex-wrap justify-center gap-4">
              <Link href="/contact" className="bg-brand-accent hover:bg-amber-600 text-white px-8 py-4 rounded-lg font-bold text-lg transition">
                Book a Free Trial
              </Link>
            </div>
          </div>
        </section>

        {/* Local Relevance Section */}
        <section className="py-20 bg-white">
          <div className="max-w-7xl mx-auto px-6 lg:px-8">
            <div className="grid md:grid-cols-2 gap-12 items-center">
              <div>
                <h2 className="text-3xl font-serif font-bold text-brand-dark mb-6">Serving Students in {city.name}</h2>
                <p className="text-gray-600 text-lg mb-6 leading-relaxed">
                  {city.description}
                </p>
                <div className="bg-brand-light p-6 rounded-xl border border-brand-accent/20">
                  <h3 className="font-bold text-brand-dark mb-2 flex items-center gap-2">
                    <i className="fa-solid fa-clock text-brand-accent"></i> Specialized {city.timezone} Timings
                  </h3>
                  <p className="text-gray-600">
                    We understand the local school schedules. Our classes are scheduled during convenient evening and weekend slots in {city.timezone}.
                  </p>
                </div>
                {city.schools && (
                  <p className="mt-6 text-sm text-gray-500 italic">
                    Already training students from: {city.schools}
                  </p>
                )}
              </div>
              <div className="bg-slate-100 rounded-2xl p-8 shadow-inner">
                <img 
                  src="https://images.unsplash.com/photo-1580541832626-2a7131ee809f?ixlib=rb-1.2.1&auto=format&fit=crop&w=1000&q=80" 
                  alt={`Online Chess Classes ${city.name}`} 
                  className="rounded-xl shadow-lg mb-6"
                />
                <div className="grid grid-cols-2 gap-4">
                  <div className="bg-white p-4 rounded-lg text-center shadow-sm">
                    <p className="text-2xl font-bold text-brand-accent">100%</p>
                    <p className="text-xs uppercase tracking-wider text-gray-500">Online</p>
                  </div>
                  <div className="bg-white p-4 rounded-lg text-center shadow-sm">
                    <p className="text-2xl font-bold text-brand-accent">FIDE</p>
                    <p className="text-xs uppercase tracking-wider text-gray-500">Coaches</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* Reusing Course Levels */}
        <section id="courses" className="py-20 bg-slate-50">
          <div className="max-w-7xl mx-auto px-6 lg:px-8">
            <div className="text-center mb-16">
              <h2 className="text-3xl font-serif font-bold text-brand-dark">Our Curriculum</h2>
              <p className="text-gray-600 mt-4">Professional training paths for {city.name} students</p>
            </div>
            <CourseLevels />
          </div>
        </section>

        {/* CTA Section */}
        <section className="py-20 bg-brand-accent text-white">
          <div className="max-w-4xl mx-auto px-6 text-center">
            <h2 className="text-3xl font-serif font-bold mb-6">Start Your Chess Journey Today</h2>
            <p className="text-xl mb-8 opacity-90">
              Join the most interactive online chess academy. Free assessments available for students in {city.name}.
            </p>
            <Link href="/contact" className="bg-brand-dark hover:bg-slate-800 text-white px-10 py-4 rounded-lg font-bold text-xl transition shadow-xl">
              Get Started Now
            </Link>
          </div>
        </section>
      </main>
      <Footer />
    </>
  );
}
