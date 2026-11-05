import type { Metadata } from "next";
import { notFound } from "next/navigation";
import PolicyDocument from "../../../components/public/PolicyDocument";
import PublicShell from "../../../components/public/PublicShell";
import { getPolicyVersion } from "../../../lib/public-data";

export const revalidate = 31_536_000;

type PolicyVersionParams = { params: Promise<{ slug: string; version: string }> };

export async function generateMetadata({ params }: PolicyVersionParams): Promise<Metadata> {
  const { slug, version } = await params;
  const policy = await getPolicyVersion(slug, version);
  if (!policy) return { title: "Policy version not found", robots: { index: false, follow: false } };
  return {
    title: `${policy.title} · ${policy.version}`,
    description: policy.summary,
    alternates: { canonical: `/policies/${encodeURIComponent(policy.slug)}/${encodeURIComponent(policy.version)}` },
  };
}

export default async function PolicyVersionPage({ params }: PolicyVersionParams) {
  const { slug, version } = await params;
  const policy = await getPolicyVersion(slug, version);
  if (!policy) notFound();
  return <PublicShell><main><PolicyDocument policy={policy} /></main></PublicShell>;
}
