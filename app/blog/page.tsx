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
    <>
      <Navbar />
      <main className="min-h-screen pt-24 pb-12 bg-slate-50">
        <div className="max-w-7xl mx-auto px-6 lg:px-8">
          <div className="text-center mb-12">
            <h1 className="text-4xl font-serif font-bold text-brand-dark mb-4">Chess Insights</h1>
            <p className="text-gray-600 max-w-2xl mx-auto">
              Tips, strategies, and academy news from our masters.
            </p>
          </div>

          <div className="grid md:grid-cols-3 gap-8">
            {posts.map((post) => (
              <Link 
                href={`/blog/${post.slug.current}`} 
                key={post.slug.current}
                className="group bg-white rounded-xl shadow-sm hover:shadow-xl transition overflow-hidden border border-slate-100"
              >
                {/* Image Section */}
                <div className="h-48 overflow-hidden bg-gray-200 relative">
                  {post.mainImage && (
                    <img 
                      src={urlFor(post.mainImage).width(800).url()} 
                      alt={post.title}
                      className="w-full h-full object-cover group-hover:scale-105 transition duration-500"
                    />
                  )}
                </div>

                {/* Content Section */}
                <div className="p-6">
                  <p className="text-xs text-brand-accent font-bold mb-2 uppercase">
                    {new Date(post.publishedAt).toLocaleDateString()}
                  </p>
                  <h2 className="text-xl font-bold text-brand-dark mb-3 group-hover:text-brand-accent transition">
                    {post.title}
                  </h2>
                  <p className="text-slate-500 text-sm line-clamp-3">
                    {post.excerpt}
                  </p>
                  <div className="mt-4 text-brand-accent font-semibold text-sm flex items-center gap-1">
                    Read Article <i className="fa-solid fa-arrow-right text-xs"></i>
                  </div>
                </div>
              </Link>
            ))}
          </div>

          {posts.length === 0 && (
            <div className="text-center py-20">
              <p className="text-gray-500">No articles published yet.</p>
            </div>
          )}
        </div>
      </main>
      <Footer />
    </>
  );
}