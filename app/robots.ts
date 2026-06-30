import { MetadataRoute } from 'next'

export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: '*',
      allow: '/',
      disallow: ['/studio/'], // Sanity Studio
    },
    sitemap: 'https://kingsgambit2700.in/sitemap.xml',
  }
}
