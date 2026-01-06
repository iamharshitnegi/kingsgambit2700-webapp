import type { Metadata } from "next";
import { Inter, Merriweather } from "next/font/google";
import "./globals.css";
import FloatingWhatsApp from './components/FloatingWhatsapp';
import { Analytics } from "@vercel/analytics/next"
import { SpeedInsights } from "@vercel/speed-insights/next"

const inter = Inter({ subsets: ["latin"], variable: "--font-inter" });
const merriweather = Merriweather({ weight: ["300", "400", "700"], subsets: ["latin"], variable: "--font-merriweather" });

export const metadata: Metadata = {
  title: "ChessMaster Academy | From Beginner to Grandmaster",
  description: "Join ChessMaster Academy for professional chess coaching.",
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
      </head>
      <body className={`${inter.variable} ${merriweather.variable} font-sans text-slate-800 bg-brand-light antialiased`}>
        {children}
        <Analytics />
        <SpeedInsights />
        <FloatingWhatsApp />
      </body>
    </html>
  );
}