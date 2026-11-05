"use client";

import Link from "next/link";
import Image from "next/image";
import type { ReactNode } from "react";
import BrandLogo from "@/app/components/BrandLogo";
import { Icon } from "@/app/components/Icon";
import "./portal.css";
import { useSession } from "@/app/lib/use-session";
import { workspaceHome } from "@/app/lib/role-routing";

export default function AuthPageShell({ children }: { children: ReactNode }) {
  const { session } = useSession();
  const home = session?.authenticated && session.user ? workspaceHome(session.user) : "/";
  return (
    <main className="auth-page auth-open-play">
      <section className="auth-panel">
        <Link href={home} className="portal-brand" aria-label="KingsGambit2700 home"><BrandLogo size={46} priority /><span>KingsGambit <em>2700</em><small>THE CHESS ACADEMY</small></span></Link>
        <div className="auth-panel-copy"><span className="eyebrow">Your academy, together</span><h2>Every lesson. <br />Every little <span>breakthrough.</span></h2><p>A quieter place to keep up with classes, build good practice habits, and see how far you have come.</p></div>
        <div className="auth-brand-art"><Image src="/brand/chess-sculpture-brown.png" alt="" width={1199} height={1312} sizes="(max-width: 860px) 1px, 45vw" /><span className="auth-art-note"><Icon name="knight" size={16} /> A good place to make your next move.</span></div>
      </section>
      <section className="auth-form-panel">{children}<Link href={home} className="auth-home-link">← Back to academy</Link></section>
    </main>
  );
}
