import type { Metadata } from "next";
import PortalResolver from "../components/portal/PortalResolver";

export const metadata: Metadata = { title: "Academy portal", robots: { index: false, follow: false } };
export default function PortalPage() { return <PortalResolver />; }
