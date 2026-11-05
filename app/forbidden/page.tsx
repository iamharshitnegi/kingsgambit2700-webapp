import type { Metadata } from "next";
import Link from "next/link";
export const metadata: Metadata = { title: "Access unavailable", robots: { index: false, follow: false } };
export default function ForbiddenPage() { return <main className="portal-center"><div className="portal-status-card"><span className="eyebrow">Access unavailable</span><h1>This account does not have permission for that view.</h1><p>Role and assignment decisions come from the academy service. Return to your portal or contact the academy if this looks wrong.</p><div><Link className="button button-dark" href="/portal">Return to my portal</Link><Link className="text-link" href="/contact">Contact the academy</Link></div></div></main>; }
