import Link from 'next/link';
import { WHATSAPP_NUMBER, INSTA_URL } from '../constants';

export default function Footer() {
  return (
    <footer className="bg-brand-cream border-t border-brand-border pt-16 pb-8">
      <div className="max-w-7xl mx-auto px-6 lg:px-8 grid md:grid-cols-4 gap-12 mb-12">
        <div className="col-span-1 md:col-span-2">
          <div className="flex items-center gap-3 mb-4">
            <img src="/logo.png" alt="KingsGambit 2700" className="h-8 md:h-12 w-auto grayscale" />
            <span className="font-serif text-[15px] md:text-lg text-brand-charcoal tracking-[0.3px] font-bold">
              KingsGambit <span className="text-brand-gold italic">2700</span>
            </span>
          </div>
          <p className="text-stone-500 text-[13px] leading-relaxed max-w-xs">
            Building the next generation of thinkers and grandmasters. Join us to master the royal game.
          </p>
          <div className="flex gap-4 mt-6">
            <a href="#" className="text-stone-400 hover:text-brand-charcoal transition"><i className="fa-brands fa-facebook fa-lg"></i></a>
            <a href={INSTA_URL} className="text-stone-400 hover:text-brand-charcoal transition"><i className="fa-brands fa-instagram fa-lg"></i></a>
            <a href="#" className="text-stone-400 hover:text-brand-charcoal transition"><i className="fa-brands fa-youtube fa-lg"></i></a>
          </div>
        </div>
        <div>
          <h4 className="text-[12px] font-bold text-brand-charcoal uppercase tracking-wider mb-4">Quick Links</h4>
          <ul className="space-y-3 text-stone-500 text-[13px]">
            <li><Link href="/" className="hover:text-brand-gold transition">Home</Link></li>
            <li><Link href="/#vision" className="hover:text-brand-gold transition">About Us</Link></li>
            <li><Link href="/#courses" className="hover:text-brand-gold transition">Courses</Link></li>
            <li><Link href="/contact" className="hover:text-brand-gold transition">Contact</Link></li>
          </ul>
        </div>
        <div>
          <h4 className="text-[12px] font-bold text-brand-charcoal uppercase tracking-wider mb-4">Contact</h4>
          <ul className="space-y-3 text-stone-500 text-[13px]">
            <li className="flex items-start gap-3">
              <i className="fa-solid fa-location-dot mt-1 text-brand-gold"></i>
              <span>123, Chess Avenue, Bandra West, Mumbai, India</span>
            </li>
            <li className="flex items-center gap-3">
              <i className="fa-brands fa-whatsapp text-brand-gold"></i>
              <a href={`https://wa.me/${WHATSAPP_NUMBER}`} className="hover:text-brand-charcoal transition">+91 98765 43210</a>
            </li>
            <li className="flex items-center gap-3">
              <i className="fa-solid fa-envelope text-brand-gold"></i>
              <a href="mailto:info@chessmaster.in" className="hover:text-brand-charcoal transition">info@chessmaster.in</a>
            </li>
          </ul>
        </div>
      </div>
      <div className="border-t border-brand-border pt-8 text-center text-stone-400 text-[11px] uppercase tracking-wider">
        &copy; {new Date().getFullYear()} KingsGambit2700. All rights reserved.
      </div>
    </footer>
  );
}