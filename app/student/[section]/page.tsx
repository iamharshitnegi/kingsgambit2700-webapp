import { notFound } from "next/navigation";
import WorkspacePage from "@/app/components/portal/workspace/WorkspacePage";
import { workspaceSections, type WorkspaceSection } from "@/app/lib/academy-types";
export default async function Page({ params }: { params: Promise<{ section: string }> }) {
  const { section } = await params;
  if (!workspaceSections.includes(section as WorkspaceSection)) notFound();
  return <WorkspacePage kind="student" section={section as WorkspaceSection} />;
}
