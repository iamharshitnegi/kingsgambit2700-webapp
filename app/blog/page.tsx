import type { Metadata } from "next";
import Link from "next/link";
import Image from "next/image";
import PublicShell from "../components/public/PublicShell";
import { client } from "@/sanity/lib/client";
import { urlFor } from "@/sanity/lib/image";

export const metadata: Metadata = { title: "Chess journal", description: "Chess ideas, academy notes, and practical learning resources from KingsGambit2700." };

type Post = { title: string; slug: { current: string }; mainImage?: Parameters<typeof urlFor>[0]; publishedAt: string; excerpt?: string };

async function getPosts(): Promise<Post[]> {
  if (!client) return [];
  try {
    return await client.fetch(`*[_type == "post" && defined(slug.current) && defined(publishedAt) && publishedAt >= "2000-01-01T00:00:00Z"] | order(publishedAt desc) { title, slug, mainImage, publishedAt, "excerpt": array::join(string::split(pt::text(body), "")[0..150], "") }`);
  } catch { return []; }
}

export default async function BlogPage() {
  const posts = await getPosts();
  return <PublicShell><main><section className="journal-hero"><span className="eyebrow">The academy journal</span><h1>Ideas worth carrying to the board.</h1><p>Practical chess learning, thoughtful game review, and editorial notes from the academy.</p></section><section className="journal-grid">{posts.length ? posts.map((post, index) => <Link href={`/blog/${post.slug.current}`} className={`journal-card ${index === 0 ? "journal-card-featured" : ""}`} key={post.slug.current}><div className="journal-image">{post.mainImage ? <Image src={urlFor(post.mainImage).width(1200).height(760).fit("crop").auto("format").url()} width={1200} height={760} sizes={index === 0 ? "(max-width: 860px) 100vw, 65vw" : "(max-width: 860px) 100vw, 50vw"} alt="" /> : <span className="journal-image-mark">♞</span>}</div><div className="journal-card-copy"><span>{new Date(post.publishedAt).toLocaleDateString("en", { day: "numeric", month: "long", year: "numeric" })}</span><h2>{post.title}</h2>{post.excerpt && <p>{post.excerpt}…</p>}<strong>Read article →</strong></div></Link>) : <div className="empty-editorial"><span className="eyebrow">Editorial library</span><h2>No reviewed article is published here yet.</h2><p>The journal stays empty when the editorial CMS is not configured or contains no valid published dates. Operational academy data never comes from Sanity.</p><Link href="/curriculum" className="button button-dark">Explore the curriculum</Link></div>}</section></main></PublicShell>;
}
