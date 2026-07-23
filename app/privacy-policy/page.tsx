'use client';

import Navbar from '../components/Navbar';
import Footer from '../components/Footer';
import Link from 'next/link';

export default function PrivacyPolicy() {
    return (
        <div className="flex flex-col min-h-screen bg-brand-cream">
            <Navbar />

            <main className="flex-grow pt-32 pb-24">
                <div className="max-w-4xl mx-auto px-6 lg:px-8">
                    <div className="text-center mb-16">
                        <div className="text-brand-gold text-xs uppercase tracking-[0.2em] font-bold mb-4">Legal Info</div>
                        <h1 className="text-4xl md:text-5xl font-serif font-medium text-brand-charcoal mb-6">Privacy Policy</h1>
                        <p className="text-stone-600 text-lg max-w-2xl mx-auto">
                            We value your privacy. Learn how we collect, use, and protect your data across our platforms.
                        </p>
                    </div>

                    <div className="bg-white rounded-md shadow-sm border border-brand-border p-8 md:p-12 text-stone-600 leading-relaxed">
                        <p className="mb-6 text-sm text-stone-400 font-medium">
                            Last Updated: {new Date().toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' })}
                        </p>
                        <p className="mb-8">
                            At KingsGambit2700, we are committed to protecting your privacy. This Privacy Policy explains how your personal information is collected, used, and shared when you visit our website or engage with our services, as well as your rights under regulations like the GDPR and CCPA.
                        </p>

                        <h2 className="text-2xl font-serif font-medium text-brand-charcoal mt-10 mb-4">1. What Personal Data We Collect</h2>
                        <p className="mb-4">When you visit our website, register for classes, or communicate with us, we may collect the following types of information:</p>
                        <ul className="list-disc pl-6 mb-8 space-y-3">
                            <li><strong className="text-brand-charcoal">Identity Data:</strong> First name, last name, and usernames.</li>
                            <li><strong className="text-brand-charcoal">Contact Data:</strong> Email address, phone number, and physical address.</li>
                            <li><strong className="text-brand-charcoal">Technical Data:</strong> Internet Protocol (IP) address, browser type and version, time zone setting, and device information.</li>
                            <li><strong className="text-brand-charcoal">Usage & Tracking Data:</strong> Information about how you use our website, pages viewed, and cookies.</li>
                        </ul>

                        <h2 className="text-2xl font-serif font-medium text-brand-charcoal mt-10 mb-4">2. Third-Party Tracking Pixels & Advertising</h2>
                        <p className="mb-4">
                            We launch paid advertising campaigns on platforms such as Meta (Facebook/Instagram), Google Ads, and TikTok to reach new students. To measure the effectiveness of these campaigns and deliver targeted ads, we use third-party tracking pixels and cookies.
                        </p>
                        <ul className="list-disc pl-6 mb-8 space-y-3">
                            <li><strong className="text-brand-charcoal">Meta Pixel:</strong> Allows us to track conversions from Facebook ads, optimize ads based on collected data, and build targeted audiences. Meta may use this behavioral data in accordance with their privacy policy.</li>
                            <li><strong className="text-brand-charcoal">Google Analytics & Google Ads:</strong> We use Google Analytics to understand website traffic and Google Ads to serve relevant advertisements. Google uses cookies to serve ads based on your prior visits to our website.</li>
                            <li><strong className="text-brand-charcoal">TikTok Pixel:</strong> Helps us track user actions on our website following a click on a TikTok ad to measure ad performance and retarget interested users.</li>
                        </ul>

                        <h2 className="text-2xl font-serif font-medium text-brand-charcoal mt-10 mb-4">3. Your Rights Under GDPR & CCPA</h2>
                        <p className="mb-4">
                            Depending on your location, you may have the following rights regarding your personal data under the General Data Protection Regulation (GDPR) and the California Consumer Privacy Act (CCPA):
                        </p>
                        <ul className="list-disc pl-6 mb-8 space-y-3">
                            <li><strong className="text-brand-charcoal">The Right to Access:</strong> You have the right to request copies of your personal data.</li>
                            <li><strong className="text-brand-charcoal">The Right to Deletion (Right to be Forgotten):</strong> You have the right to request that we erase your personal data, under certain conditions.</li>
                            <li><strong className="text-brand-charcoal">The Right to Opt-Out:</strong> You have the right to opt-out of the sale or sharing of your personal information for targeted advertising.</li>
                            <li><strong className="text-brand-charcoal">The Right to Rectification:</strong> You have the right to request that we correct any information you believe is inaccurate.</li>
                        </ul>

                        <h2 className="text-2xl font-serif font-medium text-brand-charcoal mt-10 mb-4">4. How to Opt-Out of Targeted Tracking</h2>
                        <p className="mb-4">
                            You can choose to disable cookies through your individual browser options. Additionally, you can opt-out of targeted advertising and third-party tracking by visiting the following industry-standard opt-out pages:
                        </p>
                        <ul className="list-disc pl-6 mb-8 space-y-3">
                            <li><a href="https://optout.aboutads.info/" target="_blank" rel="noopener noreferrer" className="text-brand-gold hover:text-brand-charcoal transition underline">Digital Advertising Alliance (AboutAds.info)</a></li>
                            <li><a href="https://optout.networkadvertising.org/" target="_blank" rel="noopener noreferrer" className="text-brand-gold hover:text-brand-charcoal transition underline">Network Advertising Initiative (NAI)</a></li>
                            <li>To opt-out of Google Analytics, you can install the <a href="https://tools.google.com/dlpage/gaoptout" target="_blank" rel="noopener noreferrer" className="text-brand-gold hover:text-brand-charcoal transition underline">Google Analytics Opt-out Browser Add-on</a>.</li>
                        </ul>

                        <h2 className="text-2xl font-serif font-medium text-brand-charcoal mt-10 mb-4">5. Contact Us</h2>
                        <p className="mb-8">
                            If you have any questions about this Privacy Policy or wish to exercise your rights, please <Link href="/contact" className="text-brand-gold hover:text-brand-charcoal transition underline">contact us</Link>.
                        </p>
                    </div>
                </div>
            </main>

            <Footer />
        </div>
    );
}
