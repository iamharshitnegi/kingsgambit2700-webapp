import Link from "next/link";
import PublicShell from "./components/public/PublicShell";

export default function NotFound() {
  return <PublicShell><main className="portal-center"><div className="portal-status-card"><span className="eyebrow">Position not found</span><h1>This page is no longer on the board.</h1><p>The link may be old, or the content may not be published.</p><div><Link href="/" className="button button-dark">Return home</Link><Link href="/contact" className="text-link">Ask for help</Link></div></div></main></PublicShell>;
}
