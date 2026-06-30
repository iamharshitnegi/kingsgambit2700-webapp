'use client';

import { WHATSAPP_NUMBER } from '../constants';

export default function LeadForm() {
    return (
        <form className="space-y-6" onSubmit={(e) => { e.preventDefault(); alert('Form submitted!'); }}>
            <div>
                <label className="block text-sm font-bold text-brand-charcoal mb-2">Parent's Full Name</label>
                <input type="text" className="w-full px-4 py-3 rounded-md border border-brand-border focus:ring-1 focus:ring-brand-charcoal focus:border-brand-charcoal outline-none transition text-brand-charcoal" placeholder="John Doe" required />
            </div>
            <div className="grid grid-cols-2 gap-6">
                <div>
                    <label className="block text-sm font-bold text-brand-charcoal mb-2">Phone Number</label>
                    <input type="tel" className="w-full px-4 py-3 rounded-md border border-brand-border focus:ring-1 focus:ring-brand-charcoal focus:border-brand-charcoal outline-none transition text-brand-charcoal" placeholder="+91..." required />
                </div>
                <div>
                    <label className="block text-sm font-bold text-brand-charcoal mb-2">Child's Age</label>
                    <input type="number" className="w-full px-4 py-3 rounded-md border border-brand-border focus:ring-1 focus:ring-brand-charcoal focus:border-brand-charcoal outline-none transition text-brand-charcoal" placeholder="e.g. 8" required min="4" max="18" />
                </div>
            </div>
            <div>
                <label className="block text-sm font-bold text-brand-charcoal mb-2">Current Skill Level</label>
                <select className="w-full px-4 py-3 rounded-md border border-brand-border focus:ring-1 focus:ring-brand-charcoal focus:border-brand-charcoal outline-none transition text-brand-charcoal bg-white">
                    <option>Beginner (Knows piece movements)</option>
                    <option>Intermediate (Plays regularly)</option>
                    <option>Advanced (Rated/Tournament player)</option>
                    <option>Complete Beginner</option>
                </select>
            </div>
            <button type="submit" className="w-full bg-brand-charcoal hover:bg-stone-800 text-brand-cream text-sm font-bold py-3.5 rounded-md transition shadow-sm mt-4">
                Book Your Free Trial
            </button>

            <div className="relative flex py-6 items-center">
                <div className="flex-grow border-t border-brand-border"></div>
                <span className="flex-shrink-0 mx-4 text-stone-400 text-xs font-bold tracking-widest">OR</span>
                <div className="flex-grow border-t border-brand-border"></div>
            </div>

            <a
                href={`https://wa.me/${WHATSAPP_NUMBER}?text=Hello,%20I%20would%20like%20to%20book%20a%20free%20trial`}
                target="_blank"
                className="w-full flex justify-center items-center gap-2 border border-brand-charcoal text-brand-charcoal hover:bg-black/5 text-sm font-bold py-3.5 rounded-md transition"
            >
                <i className="fa-brands fa-whatsapp text-lg text-green-600"></i> Book via WhatsApp
            </a>
        </form>
    );
}
