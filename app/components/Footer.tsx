import Link from 'next/link';
import { WHATSAPP_NUMBER, INSTA_URL } from '../constants';

export default function Footer() {
  return (
    <footer className="bg-brand-dark text-white pt-16 pb-8">
      <div className="max-w-7xl mx-auto px-6 lg:px-8 grid md:grid-cols-4 gap-12 mb-12">
        <div className="col-span-1 md:col-span-2">
          <div className="flex items-center gap-2 mb-4">
            <i className="fa-solid fa-chess-king text-brand-accent text-2xl"></i>
            <span className="font-serif text-xl font-bold">ChessMaster Academy</span>
          </div>
          <p className="text-slate-400 text-sm leading-relaxed max-w-xs">
            Building the next generation of thinkers and grandmasters. Join us to master the royal game.
          </p>
          <div className="flex gap-4 mt-6">
            <a href="#" className="text-slate-400 hover:text-white"><i className="fa-brands fa-facebook fa-lg"></i></a>
            <a href={INSTA_URL} className="text-slate-400 hover:text-white"><i className="fa-brands fa-instagram fa-lg"></i></a>
            <a href="#" className="text-slate-400 hover:text-white"><i className="fa-brands fa-youtube fa-lg"></i></a>
          </div>
        </div>
        <div>
          <h4 className="text-lg font-bold mb-4">Quick Links</h4>
          <ul className="space-y-2 text-slate-400 text-sm">
            <li><Link href="/" className="hover:text-brand-accent">Home</Link></li>
            <li><Link href="/#vision" className="hover:text-brand-accent">About Us</Link></li>
            <li><Link href="/#courses" className="hover:text-brand-accent">Courses</Link></li>
            <li><Link href="/contact" className="hover:text-brand-accent">Contact</Link></li>
          </ul>
        </div>
        <div>
          <h4 className="text-lg font-bold mb-4">Contact</h4>
          <ul className="space-y-2 text-slate-400 text-sm">
            <li className="flex items-start gap-3">
              <i className="fa-solid fa-location-dot mt-1 text-brand-accent"></i>
              <span>123, Chess Avenue, Bandra West, Mumbai, India</span>
            </li>
            <li className="flex items-center gap-3">
              <i className="fa-brands fa-whatsapp text-brand-accent"></i>
              <a href={`https://wa.me/${WHATSAPP_NUMBER}`} className="hover:text-white">+91 98765 43210</a>
            </li>
            <li className="flex items-center gap-3">
              <i className="fa-solid fa-envelope text-brand-accent"></i>
              <a href="mailto:info@chessmaster.in" className="hover:text-white">info@chessmaster.in</a>
            </li>
          </ul>
        </div>
      </div>
      <div className="border-t border-slate-800 pt-8 text-center text-slate-500 text-sm">
        &copy; 2023 ChessMaster Academy. All rights reserved.
      </div>
    </footer>
  );
}