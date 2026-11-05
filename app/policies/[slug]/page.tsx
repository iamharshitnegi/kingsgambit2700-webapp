import type { Metadata } from "next";
import { notFound } from "next/navigation";
import PublicShell from "../../components/public/PublicShell";
import PolicyDocument from "../../components/public/PolicyDocument";
import { getPolicy, getPolicySummaries } from "../../lib/public-data";

export async function generateStaticParams() {
  const policies = await getPolicySummaries();
  return policies.filter(({ reviewStatus }) => reviewStatus === "published").map(({ slug }) => ({ slug }));
}

export async function generateMetadata({ params }: { params: Promise<{ slug: string }> }): Promise<Metadata> {
  const { slug } = await params;
  const policy = await getPolicy(slug);
  if (!policy) return { title: "Policy not found", robots: { index: false, follow: false } };
  const published = policy.reviewStatus === "published";
  return {
    title: policy.title,
    description: policy.summary,
    alternates: { canonical: `/policies/${encodeURIComponent(policy.slug)}` },
    robots: published ? undefined : { index: false, follow: false },
  };
}

export default async function PolicyPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const policy = await getPolicy(slug);
  if (!policy) notFound();

  return (
    <PublicShell><main><PolicyDocument policy={policy} /></main></PublicShell>
  );
}
