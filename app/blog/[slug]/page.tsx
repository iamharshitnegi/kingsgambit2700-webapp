import { client } from '@/sanity/lib/client';
import { urlFor } from '@/sanity/lib/image';
import { PortableText } from 'next-sanity';
import Navbar from '../../components/Navbar';
import Footer from '../../components/Footer';
import Image from 'next/image';

// 1. Define the Post Interface (what data we get back)
interface Post {
  title: string;
  publishedAt: string;
  mainImage: any;
  body: any;
  authorName: string; // Assuming standard schema has author->name
}

// 2. Fetch data for a single post
async function getPost(slug: string) {
  const query = `
    *[_type == "post" && slug.current == $slug][0] {
      title,
      publishedAt,
      mainImage,
      body,
      "authorName": author->name
    }
  `;
  return client.fetch(query, { slug });
}

// 3. Define Custom Styles for the Rich Text
// This tells Next.js how to style Sanity's content using Tailwind
const ptComponents = {
  types: {
    image: ({ value }: any) => {
      if (!value?.asset?._ref) {
        return null;
      }
      return (
        <div className="relative w-full h-96 my-8 rounded-lg overflow-hidden">
          <img
            src={urlFor(value).fit('max').auto('format').url()}
            alt={value.alt || ' '}
            className="object-cover w-full h-full"
          />
        </div>
      );
    },
  },
  block: {
    h1: ({ children }: any) => <h1 className="text-3xl font-bold mt-8 mb-4 text-brand-dark">{children}</h1>,
    h2: ({ children }: any) => <h2 className="text-2xl font-bold mt-8 mb-4 text-brand-dark">{children}</h2>,
    h3: ({ children }: any) => <h3 className="text-xl font-bold mt-6 mb-3 text-brand-dark">{children}</h3>,
    normal: ({ children }: any) => <p className="mb-4 text-gray-700 leading-relaxed">{children}</p>,
    blockquote: ({ children }: any) => (
      <blockquote className="border-l-4 border-brand-accent pl-4 py-2 my-4 italic bg-slate-50 text-gray-600">
        {children}
      </blockquote>
    ),
  },
  list: {
    bullet: ({ children }: any) => <ul className="list-disc pl-5 mb-4 space-y-2 text-gray-700">{children}</ul>,
    number: ({ children }: any) => <ol className="list-decimal pl-5 mb-4 space-y-2 text-gray-700">{children}</ol>,
  },
};

// 4. The Page Component
// Notice: params is now defined as a Promise
export default async function PostPage({ params }: { params: Promise<{ slug: string }> }) {
  
  // 1. Await the params to get the actual slug
  const { slug } = await params;

  // 2. Now pass the awaited slug to your function
  const post: Post = await getPost(slug);

  if (!post) {
    return (
      <div className="text-center py-20">
        <h1 className="text-2xl font-bold text-brand-dark">Post not found</h1>
        <p className="text-gray-500 mt-2">The article you are looking for does not exist.</p>
      </div>
    );
  }

  return (
    <>
      <Navbar />
      <article className="min-h-screen pt-24 pb-20 bg-white">
        {/* Header Section */}
        <div className="max-w-3xl mx-auto px-6">
          <div className="text-center mb-8">
            <p className="text-brand-accent font-bold uppercase tracking-wide text-sm mb-2">
              {new Date(post.publishedAt).toLocaleDateString()}
            </p>
            <h1 className="text-3xl md:text-5xl font-serif font-bold text-brand-dark leading-tight mb-6">
              {post.title}
            </h1>
            {post.authorName && (
              <p className="text-gray-500 italic">By {post.authorName}</p>
            )}
          </div>

          {/* Main Hero Image */}
          {post.mainImage && (
            <div className="w-full h-[400px] relative rounded-xl overflow-hidden shadow-xl mb-12">
               <img
                  src={urlFor(post.mainImage).width(1200).url()}
                  alt={post.title}
                  className="w-full h-full object-cover"
                />
            </div>
          )}

          {/* The Actual Content */}
          <div className="prose prose-lg max-w-none">
            <PortableText value={post.body} components={ptComponents} />
          </div>
        </div>
      </article>
      <Footer />
    </>
  );
}