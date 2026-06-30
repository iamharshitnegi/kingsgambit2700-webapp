import type { Metadata } from "next";
import { Inter, Merriweather } from "next/font/google";
import "./globals.css";
import FloatingWhatsApp from './components/FloatingWhatsapp';
import { Analytics } from "@vercel/analytics/next"
import { SpeedInsights } from "@vercel/speed-insights/next"

const inter = Inter({ subsets: ["latin"], variable: "--font-inter" });
const merriweather = Merriweather({ weight: ["300", "400", "700"], subsets: ["latin"], variable: "--font-merriweather" });

export const metadata: Metadata = {
  metadataBase: new URL('https://kingsgambit2700.in'),
  title: {
    template: '%s | KingsGambit2700 Online Chess Academy',
    default: 'KingsGambit2700 | Professional Online Chess Academy for Kids',
  },
  description: "Elite online chess coaching for beginners to advanced players. FIDE-certified coaches, small batches, and interactive lessons for students in Dubai, Bangalore, and globally.",
  keywords: ["online chess classes", "chess academy for kids", "FIDE rated chess coaches", "chess coaching Dubai", "chess classes India"],
  authors: [{ name: "KingsGambit2700" }],
  openGraph: {
    title: "KingsGambit2700 | Online Chess Academy",
    description: "Master chess from home with elite FIDE coaches.",
    url: "https://kingsgambit2700.in",
    siteName: "KingsGambit2700",
    images: [
      {
        url: "/og-image.jpg",
        width: 1200,
        height: 630,
      },
    ],
    locale: "en_US",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "KingsGambit2700 | Online Chess Academy",
    description: "Master chess from home with elite FIDE coaches.",
    images: ["/og-image.jpg"],
  },
  robots: {
    index: true,
    follow: true,
  },
  verification: {
    google: process.env.NEXT_PUBLIC_GOOGLE_SITE_VERIFICATION,
  },
};

const jsonLd = {
  "@context": "https://schema.org",
  "@type": "LocalBusiness",
  "name": "KingsGambit2700 Online Chess Academy",
  "image": "https://kingsgambit2700.in/og-image.jpg",
  "@id": "https://kingsgambit2700.in",
  "url": "https://kingsgambit2700.in",
  "telephone": "+91XXXXXXXXXX",
  "address": {
    "@type": "PostalAddress",
    "streetAddress": "",
    "addressLocality": "Bengaluru",
    "addressRegion": "KA",
    "postalCode": "560001",
    "addressCountry": "IN"
  },
  "geo": {
    "@type": "GeoCoordinates",
    "latitude": 12.9716,
    "longitude": 77.5946
  },
  "areaServed": [
    { "@type": "City", "name": "Dubai" },
    { "@type": "City", "name": "Bengaluru" },
    { "@type": "City", "name": "Delhi" },
    { "@type": "City", "name": "Singapore" },
    { "@type": "City", "name": "London" }
  ],
  "openingHoursSpecification": {
    "@type": "OpeningHoursSpecification",
    "dayOfWeek": [
      "Monday",
      "Tuesday",
      "Wednesday",
      "Thursday",
      "Friday",
      "Saturday",
      "Sunday"
    ],
    "opens": "00:00",
    "closes": "23:59"
  },
  "sameAs": [
    "https://www.instagram.com/kingsgambit2700",
    "https://www.facebook.com/kingsgambit2700"
  ]
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="scroll-smooth">
      <head>
        {/* Using CDN for FontAwesome as in your original file for simplicity */}
        <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/css/all.min.css" />
        <script
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
        />
      </head>
      <body className={`${inter.variable} ${merriweather.variable} font-sans text-brand-charcoal bg-brand-cream antialiased`}>
        {children}
        <Analytics />
        <SpeedInsights />
        <FloatingWhatsApp />
      </body>
    </html>
  );
}