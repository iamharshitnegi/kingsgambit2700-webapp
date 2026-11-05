import type { Metadata } from "next";
import StudentCodeForm from "../components/forms/StudentCodeForm";
import AuthPageShell from "../components/portal/AuthPageShell";

export const metadata: Metadata = { title: "Student access", robots: { index: false, follow: false } };
export default function StudentAccessPage() { return <AuthPageShell><StudentCodeForm /></AuthPageShell>; }
