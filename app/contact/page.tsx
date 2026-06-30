'use client';

import Navbar from '../components/Navbar';
import Footer from '../components/Footer';
import { WHATSAPP_NUMBER } from '../constants';

export default function Contact() {
    return (
        <div className="flex flex-col min-h-screen bg-brand-cream">
            <Navbar />

            <main className="flex-grow pt-32 pb-24">
                <div className="text-center px-6 mb-16">
                    <div className="text-brand-gold text-xs uppercase tracking-[0.2em] font-bold mb-4">Contact Us</div>
                    <h1 className="text-5xl md:text-6xl font-serif font-medium text-brand-charcoal mb-6">Get in touch</h1>
                    <p className="text-stone-600 text-lg max-w-xl mx-auto">
                        Have questions about our courses or tournaments? Reach out via WhatsApp or fill the form below.
                    </p>
                </div>

                <div className="max-w-6xl mx-auto px-6 lg:px-8">
                    <div className="bg-white rounded-md shadow-sm border border-brand-border overflow-hidden grid md:grid-cols-2">
                        {/* Info Side */}
                        <div className="bg-brand-charcoal p-10 md:p-16 text-brand-cream flex flex-col justify-between">
                            <div>
                                <h3 className="text-2xl font-serif font-medium mb-10 text-brand-goldLight">Contact Information</h3>
                                <ul className="space-y-8">
                                    <li className="flex items-start gap-5">
                                        <div className="w-10 h-10 rounded-full border border-stone-600 flex items-center justify-center flex-shrink-0">
                                            <i className="fa-brands fa-whatsapp text-lg text-brand-goldLight"></i>
                                        </div>
                                        <div>
                                            <p className="text-xs text-stone-400 uppercase tracking-[0.1em] mb-1">WhatsApp / Phone</p>
                                            <p className="text-lg font-medium tracking-wide">+91 98765 43210</p>
                                        </div>
                                    </li>
                                    <li className="flex items-start gap-5">
                                        <div className="w-10 h-10 rounded-full border border-stone-600 flex items-center justify-center flex-shrink-0">
                                            <i className="fa-regular fa-envelope text-lg text-brand-goldLight"></i>
                                        </div>
                                        <div>
                                            <p className="text-xs text-stone-400 uppercase tracking-[0.1em] mb-1">Email</p>
                                            <p className="text-lg font-medium tracking-wide">hello@chessmaster.in</p>
                                        </div>
                                    </li>
                                    <li className="flex items-start gap-5">
                                        <div className="w-10 h-10 rounded-full border border-stone-600 flex items-center justify-center flex-shrink-0">
                                            <i className="fa-solid fa-location-dot text-lg text-brand-goldLight"></i>
                                        </div>
                                        <div>
                                            <p className="text-xs text-stone-400 uppercase tracking-[0.1em] mb-1">Location</p>
                                            <p className="text-lg font-medium tracking-wide leading-relaxed">Bandra West, Mumbai<br className="hidden sm:inline" />+ Zoom (Global)</p>
                                        </div>
                                    </li>
                                </ul>
                            </div>

                            <div className="mt-12 h-48 bg-stone-800 rounded-md overflow-hidden relative opacity-80 hover:opacity-100 transition duration-300">
                                <iframe src="https://www.google.com/maps/embed?pb=!1m18!1m12!1m3!1d30153.788252261566!2d72.81974396827346!3d19.05955962084732!2m3!1f0!2f0!3f0!3m2!1i1024!2i768!4f13.1!3m3!1m2!1s0x3be7c8e123f8d27b%3A0x437996b49a236a78!2sBandra%20West%2C%20Mumbai%2C%20Maharashtra!5e0!3m2!1sen!2sin!4v1683638290123!5m2!1sen!2sin" width="100%" height="100%" style={{ border: 0 }} loading="lazy"></iframe>
                            </div>
                        </div>

                        {/* Form Side */}
                        <div className="p-10 md:p-16">
                            <form className="space-y-6" onSubmit={(e) => { e.preventDefault(); alert('Form submitted!'); }}>
                                <div>
                                    <label className="block text-sm font-bold text-brand-charcoal mb-2">Full Name</label>
                                    <input type="text" className="w-full px-4 py-3 rounded-md border border-brand-border focus:ring-1 focus:ring-brand-charcoal focus:border-brand-charcoal outline-none transition text-brand-charcoal" placeholder="John Doe" required />
                                </div>
                                <div className="grid grid-cols-2 gap-6">
                                    <div>
                                        <label className="block text-sm font-bold text-brand-charcoal mb-2">Phone Number</label>
                                        <input type="tel" className="w-full px-4 py-3 rounded-md border border-brand-border focus:ring-1 focus:ring-brand-charcoal focus:border-brand-charcoal outline-none transition text-brand-charcoal" placeholder="+91..." required />
                                    </div>
                                    <div>
                                        <label className="block text-sm font-bold text-brand-charcoal mb-2">Course Interest</label>
                                        <select className="w-full px-4 py-3 rounded-md border border-brand-border focus:ring-1 focus:ring-brand-charcoal focus:border-brand-charcoal outline-none transition text-brand-charcoal bg-white">
                                            <option>Beginner</option>
                                            <option>Intermediate</option>
                                            <option>Advanced</option>
                                            <option>Not sure</option>
                                        </select>
                                    </div>
                                </div>
                                <div>
                                    <label className="block text-sm font-bold text-brand-charcoal mb-2">Message</label>
                                    <textarea rows={4} className="w-full px-4 py-3 rounded-md border border-brand-border focus:ring-1 focus:ring-brand-charcoal focus:border-brand-charcoal outline-none transition text-brand-charcoal" placeholder="How can we help you?"></textarea>
                                </div>
                                <button type="submit" className="w-full bg-brand-charcoal hover:bg-stone-800 text-brand-cream text-sm font-bold py-3.5 rounded-md transition shadow-sm mt-4">
                                    Send Message
                                </button>

                                <div className="relative flex py-6 items-center">
                                    <div className="flex-grow border-t border-brand-border"></div>
                                    <span className="flex-shrink-0 mx-4 text-stone-400 text-xs font-bold tracking-widest">OR</span>
                                    <div className="flex-grow border-t border-brand-border"></div>
                                </div>

                                <a
                                    href={`https://wa.me/${WHATSAPP_NUMBER}?text=Hello,%20I%20need%20more%20info`}
                                    target="_blank"
                                    className="w-full flex justify-center items-center gap-2 border border-brand-charcoal text-brand-charcoal hover:bg-black/5 text-sm font-bold py-3.5 rounded-md transition"
                                >
                                    <i className="fa-brands fa-whatsapp text-lg text-green-600"></i> Chat on WhatsApp
                                </a>
                            </form>
                        </div>
                    </div>
                </div>
            </main>

            <Footer />
        </div>
    );
}