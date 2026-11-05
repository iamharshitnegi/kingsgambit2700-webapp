import Link from "next/link";
import { isDemoMode } from "@/app/lib/demo/mode";
import "./demo.css";

export default function DemoBanner() {
  if (!isDemoMode()) return null;
  return <aside className="demo-banner" aria-label="Local demo environment"><span><strong>Local demo</strong> · Sample data. No real payments or emails.</span><Link href="/demo">Switch role / reset ↗</Link></aside>;
}
