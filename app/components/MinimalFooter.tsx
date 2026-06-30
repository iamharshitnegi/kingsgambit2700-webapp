import Link from 'next/link';

export default function MinimalFooter() {
    return (
        <footer className="bg-brand-cream py-10 text-center text-stone-500 border-t border-brand-border">
            <div className="max-w-7xl mx-auto px-6 lg:px-8 flex flex-col items-center">
                <div className="flex items-center gap-3 mb-6">
                    <img src="/logo.png" alt="KingsGambit 2700" className="h-6 w-auto grayscale" />
                    <span className="font-serif font-bold text-[15px] text-brand-charcoal tracking-[0.3px]">
                        KingsGambit <span className="text-brand-gold italic">2700</span>
                    </span>
                </div>
                <p className="text-xs uppercase tracking-wider mb-6 font-medium">
                    &copy; {new Date().getFullYear()} KingsGambit2700. All rights reserved.
                </p>
                <div className="flex gap-6 text-xs uppercase tracking-wider font-bold">
                    <Link href="#" className="hover:text-brand-charcoal transition">Privacy Policy</Link>
                    <Link href="#" className="hover:text-brand-charcoal transition">Terms of Service</Link>
                </div>
            </div>
        </footer>
    );
}
