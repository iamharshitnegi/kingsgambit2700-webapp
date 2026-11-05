"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import BrandLogo from "@/app/components/BrandLogo";
import { Icon } from "@/app/components/Icon";
import { logout } from "@/app/lib/auth-client";
import type { SessionUser } from "@/app/lib/api/types";
import { roleLabels, workspaceHome } from "@/app/lib/role-routing";
import { isDemoMode } from "@/app/lib/demo/mode";
import "./portal.css";
import "./portal-design.css";

type NavItem = { href: string; label: string; icon: string; group: string };

function navigationFor(user: SessionUser, pathname: string): NavItem[] {
  const home = workspaceHome(user, pathname);
  const staff = user.roles.some((role) =>
    ["coach", "academic_manager", "operations_admin", "super_admin"].includes(
      role,
    ),
  );
  const teaching = user.roles.some((role) =>
    ["coach", "academic_manager", "super_admin"].includes(role),
  );
  const family = user.roles.some((role) =>
    ["guardian", "student"].includes(role),
  );
  const navigation: NavItem[] = [
    { href: home, label: "Home", icon: "home", group: "Workspace" },
  ];
  const add = (
    section: string,
    label: string,
    icon: string,
    group = "Workspace",
  ) => navigation.push({ href: `${home}/${section}`, label, icon, group });
  if (staff || family) {
    add("calendar", "Calendar", "calendar");
    add("classes", "Classes", "classes");
  }
  if (
    staff ||
    user.roles.includes("guardian") ||
    user.roles.includes("finance_admin")
  )
    add("students", "Learners & batches", "users");
  if (teaching || family) {
    add("homework", home === "/student" ? "My homework" : "Homework", "homework", "Teach & learn");
    add("board", "Teaching board", "board", "Teach & learn");
    add("library", "Resource library", "book", "Teach & learn");
    add("games", "Game reviews", "knight", "Teach & learn");
    add("progress", "Progress & feedback", "chart", "Teach & learn");
  }
  if (staff || family) add("messages", "Messages", "message", "Academy");
  if (
    user.roles.some((role) =>
      ["operations_admin", "super_admin"].includes(role),
    )
  )
    add("operations", "Trials & operations", "video", "Academy");
  add("notifications", "Notifications", "bell", "Academy");
  add("settings", "Settings & access", "settings", "Academy");
  if (
    home === "/guardian" ||
    (home === "/admin" &&
      user.roles.some((role) =>
        ["operations_admin", "super_admin"].includes(role),
      ))
  )
    add("support", "Support", "help", "Academy");
  return navigation;
}

export default function PortalShell({
  user,
  title,
  eyebrow,
  description,
  children,
}: {
  user: SessionUser;
  title: string;
  eyebrow: string;
  description?: string;
  children: ReactNode;
}) {
  const pathname = usePathname();
  const classroom = pathname.startsWith("/classroom/");
  const homework = pathname.endsWith("/homework");
  const router = useRouter();
  const [menuOpen, setMenuOpen] = useState(false);
  const [compactNavigation, setCompactNavigation] = useState(false);
  const [signingOut, setSigningOut] = useState(false);
  const [logoutError, setLogoutError] = useState("");
  const menuButton = useRef<HTMLButtonElement>(null);
  const sidebar = useRef<HTMLElement>(null);
  const navigation = navigationFor(user, pathname);
  const workspaceRole = (["guardian", "student", "coach"] as const).find(
    (role) => pathname === `/${role}` || pathname.startsWith(`/${role}/`),
  );
  const home = workspaceHome(user, pathname);

  const closeNavigation = useCallback(() => {
    setMenuOpen(false);
    // The opener becomes focusable only after React removes the main area's inert attribute.
    requestAnimationFrame(() => menuButton.current?.focus());
  }, []);

  useEffect(() => {
    try {
      sessionStorage.setItem("kg2700.workspace", home);
    } catch {
      /* Optional navigation preference. */
    }
  }, [home]);

  useEffect(() => {
    function closeOnEscape(event: KeyboardEvent) {
      if (event.key === "Escape" && menuOpen) closeNavigation();
    }
    const viewport = window.matchMedia("(max-width: 860px)");
    const updateViewport = () => setCompactNavigation(viewport.matches);
    updateViewport();
    viewport.addEventListener("change", updateViewport);
    window.addEventListener("keydown", closeOnEscape);
    return () => {
      viewport.removeEventListener("change", updateViewport);
      window.removeEventListener("keydown", closeOnEscape);
    };
  }, [pathname, menuOpen, closeNavigation]);

  useEffect(() => {
    if (!menuOpen || !compactNavigation) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const links = () =>
      Array.from(
        sidebar.current?.querySelectorAll<HTMLElement>(
          "a[href], button:not(:disabled)",
        ) || [],
      ).filter((node) => node.getClientRects().length);
    links()[0]?.focus();
    function trap(event: KeyboardEvent) {
      if (event.key !== "Tab") return;
      const items = links();
      const first = items[0],
        last = items.at(-1);
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last?.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first?.focus();
      }
    }
    document.addEventListener("keydown", trap);
    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener("keydown", trap);
    };
  }, [menuOpen, compactNavigation]);

  async function handleLogout() {
    setSigningOut(true);
    setLogoutError("");
    try {
      await logout();
      router.replace("/sign-in");
    } catch {
      setSigningOut(false);
      setLogoutError("Sign out failed. Please try again.");
    }
  }

  return (
    <div
      className={`portal-layout${classroom ? " portal-layout--classroom" : ""}${homework ? " portal-layout--homework" : ""}`}
    >
      {!classroom && (
        <aside
          ref={sidebar}
          role={compactNavigation && menuOpen ? "dialog" : undefined}
          aria-modal={compactNavigation && menuOpen ? true : undefined}
          aria-label="Academy navigation"
          id="portal-navigation"
          inert={compactNavigation && !menuOpen}
          className={`portal-sidebar ${menuOpen ? "is-open" : ""}`}
        >
          <div className="portal-brand-row">
            <Link
              href={home}
              className="portal-brand"
              aria-label="KingsGambit2700 home"
            >
              <BrandLogo size={42} />
              <span>
                KingsGambit <em>2700</em>
                <small>THE CHESS ACADEMY</small>
              </span>
            </Link>
            <button
              className="portal-close"
              onClick={closeNavigation}
              aria-label="Close portal navigation"
            >
              <Icon name="close" size={20} />
            </button>
          </div>
          <div className="portal-workspace-tag">
            <span>KG</span>
            <div>
              Academy workspace
              <small>{roleLabels[workspaceRole || user.roles[0]]}</small>
            </div>
          </div>
          <nav aria-label="Portal navigation" className="portal-nav">
            {Array.from(new Set(navigation.map((item) => item.group))).map(
              (group) => (
                <div className="portal-nav-group" key={group}>
                  <p className="portal-workspace-label">{group}</p>
                  {navigation
                    .filter((item) => item.group === group)
                    .map((item) => (
                      <Link
                        key={item.href}
                        href={item.href}
                        className={pathname === item.href ? "is-active" : ""}
                        aria-current={
                          pathname === item.href ? "page" : undefined
                        }
                        onClick={() => setMenuOpen(false)}
                      >
                        <Icon name={item.icon} size={19} />
                        <span>{item.label}</span>
                      </Link>
                    ))}
                </div>
              ),
            )}
            {user.roles.length > 1 && (
              <Link href="/portal" onClick={() => setMenuOpen(false)}>
                <Icon name="grid" size={19} />
                <span>Switch workspace</span>
              </Link>
            )}
          </nav>
          <Link className="portal-help" href="/contact">
            <Icon name="help" size={18} />
            Need a hand?
          </Link>
          <div className="portal-account">
            <div className="portal-account-identity">
              <span className="portal-account-avatar" aria-hidden="true">
                {(user.displayName || user.email || "A")
                  .slice(0, 1)
                  .toUpperCase()}
              </span>
              <div>
                <span>
                  {user.displayName || user.email || "Academy account"}
                </span>
                <small>
                  {user.roles.map((role) => roleLabels[role]).join(" · ")}
                </small>
              </div>
            </div>
            <button onClick={handleLogout} disabled={signingOut}>
              {signingOut ? "Signing out…" : "Sign out"}
            </button>
            {logoutError && <small role="alert">{logoutError}</small>}
          </div>
        </aside>
      )}

      <div className="portal-main" inert={compactNavigation && menuOpen}>
        <header className="portal-topbar">
          {!classroom && (
            <button
              className="portal-menu"
              ref={menuButton}
              onClick={() => setMenuOpen(true)}
              aria-label="Open portal navigation"
              aria-expanded={menuOpen}
              aria-controls="portal-navigation"
            >
              <Icon name="menu" size={22} />
            </button>
          )}
          <Link
            href={home}
            className="portal-wordmark"
            aria-label="KingsGambit2700 home"
          >
            <BrandLogo size={30} />
            <span>KingsGambit2700</span>
          </Link>
          <Link href={home} className="portal-back-home">
            <span aria-hidden="true">←</span> Back to academy
          </Link>
          <span className="portal-breadcrumb">
            <Icon name="right" size={14} />
            {title}
          </span>
          <Link href="/" className="portal-website">
            Visit website <Icon name="external" size={14} />
          </Link>
          {isDemoMode() && (
            <Link href="/demo" className="portal-demo-pill">
              Local demo · switch role
            </Link>
          )}
          <span className="portal-role">
            {roleLabels[workspaceRole || user.roles[0]]}
          </span>
          {classroom && (
            <button
              className="portal-topbar-signout"
              disabled={signingOut}
              onClick={handleLogout}
            >
              {signingOut ? "Signing out…" : "Sign out"}
            </button>
          )}
        </header>
        <main className="portal-content">
          {!classroom && !homework && (
            <div className="portal-heading">
              <span className="eyebrow">{eyebrow}</span>
              <h1>{title}</h1>
              {description && <p>{description}</p>}
            </div>
          )}
          {classroom && logoutError && (
            <p className="form-message form-message-error" role="alert">
              {logoutError}
            </p>
          )}
          {children}
          {!classroom && (
            <footer className="portal-workspace-footer">
              <span>KingsGambit2700 · Made for the next move.</span>
              <div>
                <Link href="/policies/child-safety">Child safety</Link>
                <Link href="/privacy-policy">Privacy</Link>
                <Link href="/contact">
                  Get in touch <Icon name="diagonal" size={13} />
                </Link>
              </div>
            </footer>
          )}
        </main>
      </div>
      {menuOpen && (
        <button
          className="portal-scrim"
          onClick={closeNavigation}
          tabIndex={-1}
          aria-label="Close navigation"
        />
      )}
    </div>
  );
}
