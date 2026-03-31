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

      {/* Hero Section with Stats Overlay */}
      <section className="relative h-screen flex items-center justify-center text-center text-white">
        <div className="absolute inset-0 bg-slate-900">
          <img src="https://images.unsplash.com/photo-1529699211952-734e80c4d42b?ixlib=rb-1.2.1&auto=format&fit=crop&w=1950&q=80" alt="Chess Board" className="w-full h-full object-cover opacity-40" />
        </div>

        <div className="relative z-10 max-w-4xl px-6">
          <h1 className="text-4xl md:text-6xl font-serif font-bold mb-6 leading-tight">
            From Beginner to <span className="text-brand-accent">Grandmaster</span>
          </h1>
          <p className="text-lg md:text-xl text-gray-200 mb-8 max-w-2xl mx-auto">
            Unlock your potential with professional chess coaching. Join a community where strategy meets creativity.
          </p>
          <div className="flex flex-col sm:flex-row gap-4 justify-center">
            <Link href="/contact" className="bg-brand-accent hover:bg-amber-600 text-white px-8 py-4 rounded-lg font-bold text-lg transition shadow-lg">
              Get a Free Trial
            </Link>
            <Link href="#courses" className="border-2 border-white hover:bg-white hover:text-brand-dark text-white px-8 py-4 rounded-lg font-bold text-lg transition">
              Explore Courses
            </Link>
          </div>
        </div>

        {/* Stats Section Overlay */}
        <div className="hidden md:block absolute bottom-0 left-0 right-0 z-20">
          <StatsSection />
        </div>
      </section>

      {/* Stats Section for Mobile - Below Hero */}
      <section className="md:hidden bg-white">
        <StatsSection />
      </section>

      {/* Vision Section */}
      <section id="vision" className="py-20 bg-white">
        <div className="max-w-7xl mx-auto px-6 lg:px-8">
          <div className="grid md:grid-cols-2 gap-12 items-center">
            <div>
              <h2 className="text-3xl font-serif font-bold text-brand-dark mb-6">More Than Just a Game</h2>
              <p className="text-gray-600 mb-4 leading-relaxed">
                At KingsGambit2700, we believe chess is a tool for life. Our vision is to cultivate critical thinking, patience, and sportsmanship in every student.
              </p>
              <p className="text-gray-600 mb-6 leading-relaxed">
                Whether you are looking to compete at the national level or simply want to improve concentration, our curriculum is designed to adapt to your goals.
              </p>
              <ul className="space-y-3">
                <li className="flex items-center gap-3">
                  <i className="fa-solid fa-check-circle text-brand-accent"></i>
                  <span>Structured FIDE-rated curriculum</span>
                </li>
                <li className="flex items-center gap-3">
                  <i className="fa-solid fa-check-circle text-brand-accent"></i>
                  <span>Small batch sizes (Max 6 students)</span>
                </li>
                <li className="flex items-center gap-3">
                  <i className="fa-solid fa-check-circle text-brand-accent"></i>
                  <span>Regular internal tournaments</span>
                </li>
              </ul>
            </div>
            <div className="relative">
              <img src="https://images.unsplash.com/photo-1580541832626-2a7131ee809f?ixlib=rb-1.2.1&auto=format&fit=crop&w=1000&q=80" alt="Chess Coaching" className="rounded-lg shadow-xl" />
              <div className="absolute -bottom-6 -left-6 bg-brand-dark text-white p-6 rounded-lg shadow-lg hidden md:block">
                <p className="text-4xl font-bold text-brand-accent">500+</p>
                <p className="text-sm opacity-80">Students Trained</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Courses Section */}
      <section id="courses" className="py-20 bg-white">
        <div className="max-w-7xl mx-auto px-6 lg:px-8">
          <div className="text-center mb-16">
            <h2 className="text-3xl font-serif font-bold text-brand-dark">Our Training Levels</h2>
            <p className="text-gray-600 mt-4">Structured paths for every age and skill level</p>
          </div>

          <CourseLevels />
        </div>
      </section>

      {/* Coaches Section */}
      <section id="coaches" className="py-20 bg-white">
        <div className="max-w-3xl mx-auto px-6 lg:px-8 text-center">
          <h2 className="text-3xl font-serif font-bold text-brand-dark mb-6">Our Expert Coaches</h2>
          <p className="text-xl text-gray-600 leading-relaxed">
            We have highly experienced <strong className="text-brand-accent font-semibold">FIDE-certified coaches</strong> who cater to the unique needs of our students. From building strong foundations to mastering complex strategies, our mentors are dedicated to unlocking your highest potential.
          </p>
        </div>
      </section>

      <Footer />
    </>
  );
}