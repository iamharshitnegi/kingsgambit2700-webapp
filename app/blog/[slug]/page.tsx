import type { Metadata } from "next";
import Image from "next/image";
import { notFound } from "next/navigation";
import { PortableText } from "next-sanity";
import ChessGame from "../../components/ChessGame";
import PublicShell from "../../components/public/PublicShell";
import { client } from "@/sanity/lib/client";
import { urlFor } from "@/sanity/lib/image";

type PortableValue = Array<{ _type: string; _key?: string; [key: string]: unknown }>;
type Post = { title: string; publishedAt: string; mainImage?: Parameters<typeof urlFor>[0]; body: PortableValue; authorName?: string; excerpt?: string };

async function getPost(slug: string): Promise<Post | null> {
  if (!client) return null;
  try {
    return await client.fetch(`*[_type == "post" && slug.current == $slug && defined(publishedAt) && publishedAt >= "2000-01-01T00:00:00Z"][0] { title, publishedAt, mainImage, body, "authorName": author->name, "excerpt": array::join(string::split(pt::text(body), "")[0..160], "") }`, { slug });
  } catch { return null; }
}

export async function generateMetadata({ params }: { params: Promise<{ slug: string }> }): Promise<Metadata> {
  const { slug } = await params;
  const post = await getPost(slug);
  if (!post) return { title: "Article not found" };
  return { title: post.title, description: post.excerpt, openGraph: { title: post.title, description: post.excerpt, type: "article", publishedTime: post.publishedAt, images: post.mainImage ? [urlFor(post.mainImage).width(1200).height(630).fit("crop").url()] : [] } };
}

const components = {
  types: {
    image: ({ value }: { value: { asset?: { _ref?: string }; alt?: string } }) => value?.asset?._ref ? <figure className="article-image"><Image src={urlFor(value).width(1400).auto("format").url()} width={1400} height={900} sizes="(max-width: 800px) 100vw, 1000px" alt={value.alt || ""} />{value.alt && <figcaption>{value.alt}</figcaption>}</figure> : null,
    chess: ({ value }: { value: { pgn: string; title?: string } }) => <ChessGame pgn={value.pgn} title={value.title} />,
  },
  block: {
    h2: ({ children }: { children?: React.ReactNode }) => <h2>{children}</h2>,
    h3: ({ children }: { children?: React.ReactNode }) => <h3>{children}</h3>,
    blockquote: ({ children }: { children?: React.ReactNode }) => <blockquote>{children}</blockquote>,
  },
};

export default async function PostPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const post = await getPost(slug);
  if (!post) notFound();
  const image = post.mainImage ? urlFor(post.mainImage).width(1500).height(900).fit("crop").auto("format").url() : undefined;
  return <PublicShell><main><article className="article-page"><header><span className="eyebrow">Academy journal · {new Date(post.publishedAt).toLocaleDateString("en", { day: "numeric", month: "long", year: "numeric" })}</span><h1>{post.title}</h1>{post.authorName && <p>By {post.authorName}</p>}</header>{image && <div className="article-hero-image"><Image src={image} width={1500} height={900} sizes="(max-width: 1220px) 100vw, 1180px" alt="" priority /></div>}<div className="article-body"><PortableText value={post.body} components={components} /></div><script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify({ "@context": "https://schema.org", "@type": "BlogPosting", headline: post.title, image, datePublished: post.publishedAt, author: post.authorName ? { "@type": "Person", name: post.authorName } : undefined, description: post.excerpt }).replace(/</g, "\\u003c") }} /></article></main></PublicShell>;
}
