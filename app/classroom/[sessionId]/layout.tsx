import type { Metadata } from "next";
export const metadata: Metadata = { title: "Classroom", robots: { index: false, follow: false } };
export default function ClassroomLayout({ children }: { children: React.ReactNode }) { return children; }
