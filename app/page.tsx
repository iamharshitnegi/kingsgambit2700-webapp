import Navbar from './components/Navbar';
import Footer from './components/Footer';
import { WHATSAPP_NUMBER } from './constants';
import Link from 'next/link';

export default function Home() {
  return (
    <>
      <Navbar />

      {/* Hero Section */}
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
            <a href={`https://wa.me/${WHATSAPP_NUMBER}`} className="bg-brand-accent hover:bg-amber-600 text-white px-8 py-4 rounded-lg font-bold text-lg transition shadow-lg">
              Get a Free Trial
            </a>
            <Link href="#courses" className="border-2 border-white hover:bg-white hover:text-brand-dark text-white px-8 py-4 rounded-lg font-bold text-lg transition">
              Explore Courses
            </Link>
          </div>
        </div>
      </section>

      {/* Vision Section */}
      <section id="vision" className="py-20 bg-white">
        <div className="max-w-7xl mx-auto px-6 lg:px-8">
          <div className="grid md:grid-cols-2 gap-12 items-center">
            <div>
              <h2 className="text-3xl font-serif font-bold text-brand-dark mb-6">More Than Just a Game</h2>
              <p className="text-gray-600 mb-4 leading-relaxed">
                At ChessMaster Academy, we believe chess is a tool for life. Our vision is to cultivate critical thinking, patience, and sportsmanship in every student.
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
                  <span>Small batch sizes (Max 8 students)</span>
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
      <section id="courses" className="py-20 bg-slate-50">
        <div className="max-w-7xl mx-auto px-6 lg:px-8">
          <div className="text-center mb-16">
            <h2 className="text-3xl font-serif font-bold text-brand-dark">Our Training Levels</h2>
            <p className="text-gray-600 mt-4">Structured paths for every age and skill level</p>
          </div>

          <div className="grid md:grid-cols-3 gap-8">
            <div className="bg-white p-8 rounded-xl shadow-md hover:shadow-xl transition border-t-4 border-green-500">
              <div className="w-14 h-14 bg-green-100 rounded-full flex items-center justify-center mb-6 text-green-600 text-2xl">
                <i className="fa-solid fa-chess-pawn"></i>
              </div>
              <h3 className="text-xl font-bold mb-2">Beginner</h3>
              <p className="text-sm text-gray-500 mb-4">Age 5-10 • No prior experience</p>
              <p className="text-gray-600 mb-6 text-sm">Learn the rules, piece movements, basic checkmates, and board vision.</p>
              <Link href="/contact" className="text-green-600 font-bold hover:underline">Enquire Now →</Link>
            </div>

            <div className="bg-white p-8 rounded-xl shadow-md hover:shadow-xl transition border-t-4 border-brand-accent transform md:-translate-y-4 relative">
              <div className="absolute top-0 right-0 bg-brand-accent text-white text-xs px-3 py-1 rounded-bl-lg font-bold">POPULAR</div>
              <div className="w-14 h-14 bg-amber-100 rounded-full flex items-center justify-center mb-6 text-brand-accent text-2xl">
                <i className="fa-solid fa-chess-knight"></i>
              </div>
              <h3 className="text-xl font-bold mb-2">Intermediate</h3>
              <p className="text-sm text-gray-500 mb-4">Rating &lt; 1200 • Tactics focus</p>
              {/* The fix is below: class -> className */}
              <p className="text-gray-600 mb-6 text-sm">Mastering tactics (pins, forks), opening principles, and endgame basics.</p>
              <Link href="/contact" className="text-brand-accent font-bold hover:underline">Enquire Now →</Link>
            </div>

            <div className="bg-white p-8 rounded-xl shadow-md hover:shadow-xl transition border-t-4 border-red-500">
              <div className="w-14 h-14 bg-red-100 rounded-full flex items-center justify-center mb-6 text-red-600 text-2xl">
                <i className="fa-solid fa-chess-queen"></i>
              </div>
              <h3 className="text-xl font-bold mb-2">Advanced</h3>
              <p className="text-sm text-gray-500 mb-4">Rating 1400+ • Strategy focus</p>
              <p className="text-gray-600 mb-6 text-sm">Deep opening repertoire, positional play, complex calculation, and tournament prep.</p>
              <Link href="/contact" className="text-red-600 font-bold hover:underline">Enquire Now →</Link>
            </div>
          </div>
        </div>
      </section>

      {/* Coaches Section */}
      <section id="coaches" className="py-20 bg-white">
        <div className="max-w-7xl mx-auto px-6 lg:px-8">
          <h2 className="text-3xl font-serif font-bold text-brand-dark text-center mb-12">Meet Your Mentors</h2>
          <div className="grid md:grid-cols-4 gap-8 text-center">
            {/* Coach Items (Repetitive structure optimized slightly for loops, but keeping explicit for now) */}
            <div className="group">
              <div className="relative w-40 h-40 mx-auto mb-4 overflow-hidden rounded-full border-4 border-slate-100 shadow-inner">
                <img src="https://randomuser.me/api/portraits/men/32.jpg" alt="Coach" className="w-full h-full object-cover group-hover:scale-110 transition duration-500" />
              </div>
              <h3 className="text-lg font-bold text-brand-dark">Arjun Mehta</h3>
              <p className="text-brand-accent text-sm font-semibold">International Master</p>
            </div>
            {/* ... (Add other coaches similarly) ... */}
          </div>
        </div>
      </section>

      <Footer />
    </>
  );
}