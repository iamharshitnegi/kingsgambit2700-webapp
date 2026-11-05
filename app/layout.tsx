import type { Metadata } from "next";
import "../public/brand/fonts.css";
import "./globals.css";
import "./brand.css";
import { Analytics } from "@vercel/analytics/next";
import { SpeedInsights } from "@vercel/speed-insights/next";
import { getOrganization } from "./lib/public-data";
import { getSiteURL } from "./lib/env";
import DemoBanner from "./components/demo/DemoBanner";
import SkipLink from "./components/SkipLink";
import { isDemoMode } from "./lib/demo/mode";
import { brandConfig, getBrandStyles } from "./lib/brand";

const siteURL = getSiteURL();

export const metadata: Metadata = {
  metadataBase: new URL(siteURL),
  title: { template: "%s | KingsGambit2700", default: "KingsGambit2700 | Live online chess coaching" },
  description: "Live online chess coaching with small-group and one-to-one learning paths, a clear curriculum, and a thoughtful first assessment.",
  applicationName: "KingsGambit2700",
  authors: [{ name: "KingsGambit2700" }],
  openGraph: { type: "website", locale: "en", url: siteURL, siteName: "KingsGambit2700", title: "KingsGambit2700 | A clearer path to stronger chess", description: "Live online chess coaching built around thoughtful instruction, purposeful practice, and visible learning outcomes.", images: [{ url: "/og.png", width: 1731, height: 909, alt: "KingsGambit2700 online chess coaching" }] },
  twitter: { card: "summary_large_image", title: "KingsGambit2700 | A clearer path to stronger chess", description: "Live online chess coaching with a thoughtful first assessment.", images: ["/og.png"] },
  robots: { index: true, follow: true },
  verification: { google: process.env.NEXT_PUBLIC_GOOGLE_SITE_VERIFICATION },
};

export default async function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  const organization = await getOrganization();
  const structuredData = {
    "@context": "https://schema.org",
    "@type": "EducationalOrganization",
    name: organization.name,
    url: organization.siteUrl || siteURL,
    email: organization.contactEmail,
    telephone: organization.whatsappNumber,
    sameAs: [organization.instagramUrl, organization.facebookUrl, organization.youtubeUrl].filter(Boolean),
  };

  return (
    <html lang="en" data-scroll-behavior="smooth" data-brand={brandConfig.defaultAccent} style={getBrandStyles()}>
      <body>
        <SkipLink />
        <DemoBanner />
        <div id="main-content">{children}</div>
        {!isDemoMode() && <Analytics />}
        {!isDemoMode() && <SpeedInsights />}
        <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(structuredData).replace(/</g, "\\u003c") }} />
      </body>
    </html>
  );
}
