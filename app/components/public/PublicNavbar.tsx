"use client";

import Link from "next/link";
import BrandLogo from "@/app/components/BrandLogo";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import type { OrganizationSettings } from "@/app/lib/api/types";
import { useSession } from "@/app/lib/use-session";
import { workspaceHome } from "@/app/lib/role-routing";

const links = [
  { href: "/programs", label: "Programs" },
  { href: "/curriculum", label: "Curriculum" },
  { href: "/coaches", label: "Coaches" },
  { href: "/pricing", label: "Pricing" },
  { href: "/blog", label: "Journal" },
];

export default function PublicNavbar({ organization }: { organization: OrganizationSettings }) {
  const [open, setOpen] = useState(false);
  const [scrolled, setScrolled] = useState(false);
  const headerRef = useRef<HTMLElement>(null);
  const toggleRef = useRef<HTMLButtonElement>(null);
  const pathname = usePathname();
  const { session, error } = useSession();
  const home = session?.authenticated && session.user ? workspaceHome(session.user) : "/";
  const accountHref = session?.authenticated ? home : session?.mfaRequired ? "/auth/complete?mfa=required" : session && !error ? "/sign-in" : "/portal";
  const accountLabel = session?.authenticated ? "My portal" : session?.mfaRequired ? "Complete sign-in" : session && !error ? "Sign in" : "My account";

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 16);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  useEffect(() => {
    if (!open) return;

    function onKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setOpen(false);
        toggleRef.current?.focus();
      }
    }

    function onPointerDown(event: PointerEvent) {
      if (event.target instanceof Node && !headerRef.current?.contains(event.target)) {
        setOpen(false);
      }
    }

    document.addEventListener("keydown", onKeyDown);
    document.addEventListener("pointerdown", onPointerDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      document.removeEventListener("pointerdown", onPointerDown);
    };
  }, [open]);

  return (
    <header
      ref={headerRef}
      className={`site-header ${scrolled ? "is-scrolled" : ""}`}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) setOpen(false);
      }}
    >
      <div className="site-header-inner">
        <Link href={home} className="brand-lockup" aria-label={`${organization.name} home`}>
          <BrandLogo size={48} priority />
          <span translate="no">KingsGambit <em>2700</em><small>The chess academy</small></span>
        </Link>
        <nav id="primary-navigation" className={`primary-nav ${open ? "is-open" : ""}`} aria-label="Primary navigation">
          {links.map((link) => (
            <Link key={link.href} href={link.href} aria-current={pathname === link.href ? "page" : undefined} onClick={() => setOpen(false)}>
              {link.label}
            </Link>
          ))}
          <Link href="/contact" aria-current={pathname === "/contact" ? "page" : undefined} onClick={() => setOpen(false)}>Contact</Link>
          <div className="mobile-nav-actions">
            <Link href={accountHref} className="button button-quiet" onClick={() => setOpen(false)}>{accountLabel}</Link>
            <Link href="/free-trial" className="button button-dark" onClick={() => setOpen(false)}>Book a trial</Link>
          </div>
        </nav>
        <div className="desktop-nav-actions">
          <Link href={accountHref} className="text-link">{accountLabel} <span aria-hidden="true">↗</span></Link>
          <Link href="/free-trial" className="button button-dark">Book a trial <span aria-hidden="true">→</span></Link>
        </div>
        <button
          ref={toggleRef}
          type="button"
          className={`nav-toggle ${open ? "is-open" : ""}`}
          aria-expanded={open}
          aria-controls="primary-navigation"
          aria-label={open ? "Close navigation" : "Open navigation"}
          onClick={() => setOpen((value) => !value)}
        >
          <span />
          <span />
        </button>
      </div>
    </header>
  );
}
