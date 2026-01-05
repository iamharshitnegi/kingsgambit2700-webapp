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
    <nav className={`fixed w-full z-50 transition-all duration-300 ${isScrolled ? 'bg-brand-dark shadow-xl bg-opacity-95' : 'bg-brand-dark shadow-lg'}`}>
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div className="flex justify-between h-20 items-center">
          {/* Logo */}
          <Link href="/" className="flex-shrink-0 flex items-center gap-2 cursor-pointer">
            <i className="fa-solid fa-chess-king text-brand-accent text-3xl"></i>
            <span className="font-serif text-xl font-bold tracking-wide text-white">
              ChessMaster<span className="text-brand-accent">Academy</span>
            </span>
          </Link>

          {/* Desktop Menu */}
          <div className="hidden md:flex items-center space-x-8 text-white">
            <Link href="/#vision" className="hover:text-brand-accent transition">Vision</Link>
            <Link href="/#courses" className="hover:text-brand-accent transition">Courses</Link>
            <Link href="/#coaches" className="hover:text-brand-accent transition">Coaches</Link>
            <Link href="/contact" className="hover:text-brand-accent transition">Contact</Link>
            <a 
              href={`https://wa.me/${WHATSAPP_NUMBER}?text=I'm%20interested%20in%20a%20trial%20class`} 
              target="_blank" 
              className="bg-brand-accent hover:bg-amber-600 text-white px-5 py-2 rounded-full font-semibold transition shadow-md flex items-center gap-2"
            >
              <i className="fa-brands fa-whatsapp"></i> Book Free Trial
            </a>
          </div>

          {/* Mobile Menu Button */}
          <div className="md:hidden flex items-center">
            <button onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)} className="text-white focus:outline-none">
              <i className="fa-solid fa-bars text-2xl"></i>
            </button>
          </div>
        </div>
      </div>

      {/* Mobile Menu */}
      <div className={`md:hidden bg-brand-primary border-t border-slate-700 ${isMobileMenuOpen ? 'block' : 'hidden'}`}>
        <Link href="/#vision" onClick={() => setIsMobileMenuOpen(false)} className="block py-3 px-4 text-sm text-white hover:bg-slate-700">Vision</Link>
        <Link href="/#courses" onClick={() => setIsMobileMenuOpen(false)} className="block py-3 px-4 text-sm text-white hover:bg-slate-700">Courses</Link>
        <Link href="/#coaches" onClick={() => setIsMobileMenuOpen(false)} className="block py-3 px-4 text-sm text-white hover:bg-slate-700">Coaches</Link>
        <Link href="/contact" onClick={() => setIsMobileMenuOpen(false)} className="block py-3 px-4 text-sm text-white hover:bg-slate-700">Contact Us</Link>
      </div>
    </nav>
  );
}