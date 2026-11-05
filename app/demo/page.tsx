import type { Metadata } from "next";
import { notFound } from "next/navigation";
import DemoLauncher from "../components/demo/DemoLauncher";
import { isDemoMode } from "../lib/demo/mode";

export const metadata: Metadata = { title: "Local demo", robots: { index: false, follow: false } };

export default function DemoPage() {
  if (!isDemoMode()) notFound();
  return <DemoLauncher />;
}
