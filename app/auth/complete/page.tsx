import type { Metadata } from "next";
import { Suspense } from "react";
import AuthComplete from "../../components/portal/AuthComplete";
import AuthPageShell from "../../components/portal/AuthPageShell";

export const metadata: Metadata = { title: "Completing sign in", robots: { index: false, follow: false } };
export default function AuthCompletePage() { return <AuthPageShell><Suspense fallback={<div className="auth-success"><h1>Completing secure sign-in…</h1></div>}><AuthComplete /></Suspense></AuthPageShell>; }
