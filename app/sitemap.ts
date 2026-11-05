import type { MetadataRoute } from "next";
import { client } from "@/sanity/lib/client";
import { getPolicySummaries } from "./lib/public-data";
import { getSiteURL } from "./lib/env";

const siteURL = getSiteURL();

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const policies = await getPolicySummaries();
  const publishedPolicies = policies.filter(({ reviewStatus }) => reviewStatus === "published");
  let posts: Array<{ slug: string; updatedAt: string }> = [];
  if (client) {
    try {
      posts = await client.fetch(`*[_type == "post" && defined(slug.current) && defined(publishedAt) && publishedAt >= "2000-01-01T00:00:00Z"] { "slug": slug.current, "updatedAt": _updatedAt }`);
    } catch { posts = []; }
  }

  const publicRoutes = [
    ["", "weekly", 1], ["/programs", "weekly", .9], ["/curriculum", "weekly", .9],
    ["/coaches", "weekly", .8], ["/pricing", "weekly", .8], ["/free-trial", "daily", .9],
    ["/contact", "monthly", .6], ["/blog", "weekly", .7],
  ] as const;
  const cities = ["bengaluru", "delhi", "dubai", "london", "singapore"];
  const now = new Date();

  return [
    ...publicRoutes.map(([path, changeFrequency, priority]) => ({ url: `${siteURL}${path}`, lastModified: now, changeFrequency, priority })),
    ...cities.map((city) => ({ url: `${siteURL}/online-chess-classes-${city}`, lastModified: now, changeFrequency: "monthly" as const, priority: .7 })),
    ...publishedPolicies.flatMap((policy) => {
      const lastModified = policy.effectiveAt ? new Date(policy.effectiveAt) : now;
      const slug = encodeURIComponent(policy.slug);
      const version = encodeURIComponent(policy.version);
      return [
        { url: `${siteURL}/policies/${slug}`, lastModified, changeFrequency: "monthly" as const, priority: .5 },
        { url: `${siteURL}/policies/${slug}/${version}`, lastModified, changeFrequency: "yearly" as const, priority: .4 },
      ];
    }),
    ...posts.map((post) => ({ url: `${siteURL}/blog/${post.slug}`, lastModified: new Date(post.updatedAt), changeFrequency: "monthly" as const, priority: .65 })),
  ];
}
