'use client';

import Navbar from '../components/Navbar';
import Footer from '../components/Footer';
import { WHATSAPP_NUMBER } from '../constants';

export default function Contact() {
    return (
        <div className="flex flex-col min-h-screen">
            <Navbar />

            <main className="flex-grow pt-20">
                <div className="bg-brand-primary py-16 text-center text-white">
                    <h1 className="text-4xl font-serif font-bold mb-4">Get in Touch</h1>
                    <p className="text-slate-300 max-w-xl mx-auto">Have questions about our courses or tournaments? Reach out via WhatsApp or fill the form below.</p>
                </div>

                <div className="max-w-7xl mx-auto px-6 lg:px-8 -mt-10 mb-20">
                    <div className="bg-white rounded-xl shadow-xl overflow-hidden grid md:grid-cols-2">
                        {/* Info Side */}
                        <div className="bg-brand-dark p-10 text-white flex flex-col justify-between">
                            <div>
                                <h3 className="text-2xl font-bold mb-6 text-brand-accent">Contact Information</h3>
                                <ul className="space-y-6">
                                    <li className="flex items-start gap-4">
                                        <div className="w-10 h-10 rounded-full bg-white/10 flex items-center justify-center flex-shrink-0">
                                            <i className="fa-brands fa-whatsapp text-xl text-green-400"></i>
                                        </div>
                                        <div>
                                            <p className="text-xs text-slate-400 uppercase tracking-wider">WhatsApp / Phone</p>
                                            <p className="text-lg font-semibold">+91 98765 43210</p>
                                        </div>
                                    </li>
                                    <li className="flex items-start gap-4">
                                        <div className="w-10 h-10 rounded-full bg-white/10 flex items-center justify-center flex-shrink-0">
                                            <i className="fa-regular fa-envelope text-xl text-brand-accent"></i>
                                        </div>
                                        <div>
                                            <p className="text-xs text-slate-400 uppercase tracking-wider">Email</p>
                                            <p className="text-lg font-semibold">hello@chessmaster.in</p>
                                        </div>
                                    </li>
                                    <li className="flex items-start gap-4">
                                        <div className="w-10 h-10 rounded-full bg-white/10 flex items-center justify-center flex-shrink-0">
                                            <i className="fa-solid fa-location-dot text-xl text-brand-accent"></i>
                                        </div>
                                        <div>
                                            <p className="text-xs text-slate-400 uppercase tracking-wider">Location</p>
                                            <p className="text-lg font-semibold">Bandra West, Mumbai (Offline)<br className="hidden sm:inline" />+ Zoom (Global)</p>
                                        </div>
                                    </li>
                                </ul>
                            </div>

                            <div className="mt-10 h-48 bg-slate-700 rounded-lg overflow-hidden relative group">
                                <iframe src="https://www.google.com/maps/embed?pb=!1m18!1m12!1m3!1d30153.788252261566!2d72.81974396827346!3d19.05955962084732!2m3!1f0!2f0!3f0!3m2!1i1024!2i768!4f13.1!3m3!1m2!1s0x3be7c8e123f8d27b%3A0x437996b49a236a78!2sBandra%20West%2C%20Mumbai%2C%20Maharashtra!5e0!3m2!1sen!2sin!4v1683638290123!5m2!1sen!2sin" width="100%" height="100%" style={{ border: 0 }} loading="lazy"></iframe>
                            </div>
                        </div>

                        {/* Form Side */}
                        <div className="p-10">
                            <form className="space-y-6" onSubmit={(e) => { e.preventDefault(); alert('Form submitted!'); }}>
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 mb-1">Full Name</label>
                                    <input type="text" className="w-full px-4 py-3 rounded-lg border border-gray-300 focus:ring-2 focus:ring-brand-accent focus:border-transparent outline-none transition" placeholder="John Doe" required />
                                </div>
                                <div className="grid grid-cols-2 gap-4">
                                    <div>
                                        <label className="block text-sm font-medium text-gray-700 mb-1">Phone Number</label>
                                        <input type="tel" className="w-full px-4 py-3 rounded-lg border border-gray-300 focus:ring-2 focus:ring-brand-accent focus:border-transparent outline-none transition" placeholder="+91..." required />
                                    </div>
                                    <div>
                                        <label className="block text-sm font-medium text-gray-700 mb-1">Course Interest</label>
                                        <select className="w-full px-4 py-3 rounded-lg border border-gray-300 focus:ring-2 focus:ring-brand-accent focus:border-transparent outline-none transition">
                                            <option>Beginner</option>
                                            <option>Intermediate</option>
                                            <option>Advanced</option>
                                            <option>Not sure</option>
                                        </select>
                                    </div>
                                </div>
                                <div>
                                    <label className="block text-sm font-medium text-gray-700 mb-1">Message</label>
                                    <textarea rows={4} className="w-full px-4 py-3 rounded-lg border border-gray-300 focus:ring-2 focus:ring-brand-accent focus:border-transparent outline-none transition" placeholder="How can we help you?"></textarea>
                                </div>
                                <button type="submit" className="w-full bg-brand-accent hover:bg-amber-600 text-white font-bold py-3 rounded-lg transition shadow-lg">
                                    Send Message
                                </button>

                                <div className="relative flex py-2 items-center">
                                    <div className="flex-grow border-t border-gray-300"></div>
                                    <span className="flex-shrink-0 mx-4 text-gray-400 text-sm">OR</span>
                                    <div className="flex-grow border-t border-gray-300"></div>
                                </div>

                                <a
                                    href={`https://wa.me/${WHATSAPP_NUMBER}?text=Hello,%20I%20need%20more%20info`}
                                    target="_blank"
                                    className="w-full flex justify-center items-center gap-2 bg-green-500 hover:bg-green-600 text-white font-bold py-3 rounded-lg transition shadow-lg"
                                >
                                    <i className="fa-brands fa-whatsapp text-xl"></i> Chat on WhatsApp
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