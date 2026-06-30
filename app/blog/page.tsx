import Link from 'next/link';
import { client } from '@/sanity/lib/client';
import { urlFor } from '@/sanity/lib/image';
import Navbar from '../components/Navbar';
import Footer from '../components/Footer';

// 1. Define the shape of our data (TypeScript interface)
interface Post {
  title: string;
  slug: { current: string };
  mainImage: any;
  publishedAt: string;
  excerpt: string;
}

// 2. Fetch data function
async function getPosts() {
  // GROQ query to fetch posts, sorted by newest first
  const query = `
    *[_type == "post"] | order(publishedAt desc) {
      title,
      slug,
      mainImage,
      publishedAt,
      "excerpt": array::join(string::split((pt::text(body)), "")[0..100], "") + "..."
    }
  `;
  return client.fetch(query);
}

// 3. The Page Component
export default async function BlogPage() {
  const posts: Post[] = await getPosts();

  return (
    <div className="flex flex-col min-h-screen bg-brand-cream">
      <Navbar />

      <main className="flex-grow pt-32 pb-24">
        <div className="max-w-7xl mx-auto px-6 lg:px-8">
          <div className="text-center mb-16">
            <div className="text-brand-gold text-xs uppercase tracking-[0.2em] font-bold mb-4">Our Journal</div>
            <h1 className="text-5xl md:text-6xl font-serif font-medium text-brand-charcoal mb-6">Chess Insights</h1>
            <p className="text-stone-600 text-lg max-w-2xl mx-auto">
              Tips, strategies, and academy news from our masters.
            </p>
          </div>

          <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-10 max-w-6xl mx-auto">
            {posts.map((post) => (
              <Link
                href={`/blog/${post.slug.current}`}
                key={post.slug.current}
                className="group bg-white rounded-md shadow-sm hover:shadow-md transition duration-300 overflow-hidden border border-brand-border flex flex-col"
              >
                {/* Image Section */}
                <div className="h-56 overflow-hidden bg-stone-100 relative">
                  {post.mainImage && (
                    <img
                      src={urlFor(post.mainImage).width(800).url()}
                      alt={post.title}
                      className="w-full h-full object-cover mix-blend-multiply group-hover:scale-105 transition duration-500"
                    />
                  )}
                </div>

                {/* Content Section */}
                <div className="p-8 flex-grow flex flex-col">
                  <p className="text-[10px] text-brand-gold font-bold mb-3 uppercase tracking-[0.1em]">
                    {new Date(post.publishedAt).toLocaleDateString()}
                  </p>
                  <h2 className="text-xl font-serif font-medium text-brand-charcoal mb-4 group-hover:text-brand-gold transition leading-snug">
                    {post.title}
                  </h2>
                  <p className="text-stone-500 text-sm leading-relaxed line-clamp-3 mb-6">
                    {post.excerpt}
                  </p>
                  <div className="mt-auto text-brand-charcoal font-medium text-[13px] uppercase tracking-wide flex items-center gap-2 group-hover:text-brand-gold transition">
                    Read Article <i className="fa-solid fa-arrow-right text-xs"></i>
                  </div>
                </div>
              </Link>
            ))}
          </div>

          {posts.length === 0 && (
            <div className="text-center py-20 border border-dashed border-brand-border max-w-3xl mx-auto rounded-md">
              <p className="text-stone-500 font-medium">No articles published yet.</p>
            </div>
          )}
        </div>
      </main>

      <Footer />
    </div>
  );
}