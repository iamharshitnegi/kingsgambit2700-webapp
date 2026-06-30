'use client'; // Required for interactivity (scroll/click)

import { useState, useEffect } from 'react';
import Link from 'next/link';
import { WHATSAPP_NUMBER } from '../constants';

export default function Navbar() {
  const [isScrolled, setIsScrolled] = useState(false);
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

  // Scroll effect logic
  useEffect(() => {
    const handleScroll = () => {
      setIsScrolled(window.scrollY > 50);
    };
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  return (
    <nav className={`fixed w-full z-50 transition-all duration-300 ${isScrolled ? 'bg-brand-cream/90 backdrop-blur-md shadow-sm border-b border-brand-border' : 'bg-transparent border-b border-brand-border'}`}>
      <div className="max-w-7xl mx-auto px-6 lg:px-8">
        <div className="flex justify-between items-center py-5">
          {/* Logo */}
          <Link href="/" className="flex items-center gap-3 font-serif text-xl md:text-2xl text-brand-charcoal tracking-[0.3px] font-bold">
            <img src="/logo.png" alt="KingsGambit 2700" className="h-10 md:h-14 w-auto" />
            KingsGambit <span className="text-brand-gold italic">2700</span>
          </Link>

          {/* Desktop Menu */}
          <div className="hidden md:flex items-center gap-8">
            <Link href="/#vision" className="text-stone-600 text-sm font-medium hover:text-brand-charcoal transition">Vision</Link>
            <Link href="/#courses" className="text-stone-600 text-sm font-medium hover:text-brand-charcoal transition">Courses</Link>
            <Link href="/#coaches" className="text-stone-600 text-sm font-medium hover:text-brand-charcoal transition">Coaches</Link>
            <Link href="/contact" className="text-stone-600 text-sm font-medium hover:text-brand-charcoal transition">Contact</Link>
            <Link href="/blog" className="text-stone-600 text-sm font-medium hover:text-brand-charcoal transition">Blog</Link>
            <a 
              href={`https://wa.me/${WHATSAPP_NUMBER}?text=I'm%20interested%20in%20a%20trial%20class`} 
              target="_blank" 
              className="border border-brand-charcoal text-brand-charcoal text-sm font-medium px-5 py-2.5 rounded-md hover:bg-brand-charcoal hover:text-brand-cream transition"
            >
              Book a trial
            </a>
          </div>

          {/* Mobile Menu Button */}
          <div className="md:hidden flex items-center">
            <button onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)} className="text-brand-charcoal focus:outline-none">
              <i className="fa-solid fa-bars text-xl"></i>
            </button>
          </div>
        </div>
      </div>

      {/* Mobile Menu */}
      <div className={`md:hidden bg-brand-cream border-t border-brand-border ${isMobileMenuOpen ? 'block' : 'hidden'}`}>
        <Link href="/#vision" onClick={() => setIsMobileMenuOpen(false)} className="block py-4 px-6 text-base font-medium text-stone-600 border-b border-brand-border/50">Vision</Link>
        <Link href="/#courses" onClick={() => setIsMobileMenuOpen(false)} className="block py-4 px-6 text-base font-medium text-stone-600 border-b border-brand-border/50">Courses</Link>
        <Link href="/#coaches" onClick={() => setIsMobileMenuOpen(false)} className="block py-4 px-6 text-base font-medium text-stone-600 border-b border-brand-border/50">Coaches</Link>
        <Link href="/contact" onClick={() => setIsMobileMenuOpen(false)} className="block py-4 px-6 text-base font-medium text-stone-600 border-b border-brand-border/50">Contact Us</Link>
        <Link href="/blog" onClick={() => setIsMobileMenuOpen(false)} className="block py-4 px-6 text-base font-medium text-stone-600 border-b border-brand-border/50">Blog</Link>
      </div>
    </nav>
  );
}