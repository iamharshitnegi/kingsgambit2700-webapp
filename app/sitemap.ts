import { MetadataRoute } from 'next'
import { client } from '@/sanity/lib/client'

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const baseUrl = 'https://kingsgambit2700.in'

  // Fetch blog posts from Sanity
  const postsQuery = `*[_type == "post"] { "slug": slug.current, _updatedAt }`
  const posts = await client.fetch(postsQuery)

  const blogEntries = posts.map((post: any) => ({
    url: `${baseUrl}/blog/${post.slug}`,
    lastModified: new Date(post._updatedAt),
    changeFrequency: 'weekly',
    priority: 0.7,
  }))

  // Define location pages
  const cities = ['dubai', 'bengaluru', 'delhi', 'singapore', 'london']
  const locationEntries = cities.map((city) => ({
    url: `${baseUrl}/online-chess-classes-${city}`,
    lastModified: new Date(),
    changeFrequency: 'monthly',
    priority: 0.8,
  }))

  return [
    {
      url: baseUrl,
      lastModified: new Date(),
      changeFrequency: 'daily',
      priority: 1,
    },
    {
      url: `${baseUrl}/blog`,
      lastModified: new Date(),
      changeFrequency: 'daily',
      priority: 0.9,
    },
    {
      url: `${baseUrl}/contact`,
      lastModified: new Date(),
      changeFrequency: 'monthly',
      priority: 0.5,
    },
    ...blogEntries,
    ...locationEntries,
  ]
}
