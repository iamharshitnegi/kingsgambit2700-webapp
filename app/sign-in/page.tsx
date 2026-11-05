import type { Metadata } from "next";
import { Suspense } from "react";
import SignInForm from "../components/forms/SignInForm";
import AuthPageShell from "../components/portal/AuthPageShell";

export const metadata: Metadata = { title: "Sign in", robots: { index: false, follow: false } };
export default function SignInPage() { return <AuthPageShell><Suspense fallback={<div className="auth-success"><h1>Preparing secure sign-in…</h1></div>}><SignInForm /></Suspense></AuthPageShell>; }
