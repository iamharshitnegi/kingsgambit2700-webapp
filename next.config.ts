import type { NextConfig } from "next";

const securityHeaders = [
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "X-Frame-Options", value: "SAMEORIGIN" },
  { key: "Permissions-Policy", value: "camera=(self), microphone=(self), geolocation=()" },
];

const protectedRoutes = ["/demo", "/portal/:path*", "/guardian/:path*", "/student/:path*", "/coach/:path*", "/admin/:path*", "/classroom/:path*", "/auth/:path*", "/student-access", "/sign-in", "/forbidden"];

const nextConfig: NextConfig = {
  output: process.env.KG_NEXT_STANDALONE === "true" ? "standalone" : undefined,
  poweredByHeader: false,
  distDir: process.env.KG_NEXT_DIST_DIR || ".next",
  reactStrictMode: true,
  experimental: { useTypeScriptCli: false, workerThreads: true },
  images: {
    remotePatterns: [{ protocol: "https", hostname: "cdn.sanity.io", pathname: "/images/**" }],
  },
  async headers() {
    return [
      { source: "/:path*", headers: securityHeaders },
      { source: "/sign-in", headers: [
        { key: "Cross-Origin-Opener-Policy", value: "same-origin-allow-popups" },
        ...(process.env.NODE_ENV === "development" ? [{ key: "Referrer-Policy", value: "no-referrer-when-downgrade" }] : []),
      ] },
      { source: "/policies/:slug/:version", headers: [{ key: "Cache-Control", value: "public, max-age=31536000, immutable" }] },
      { source: "/classroom/:path*", headers: [{ key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" }] },
      ...protectedRoutes.map((source) => ({ source, headers: [{ key: "Cache-Control", value: "private, no-store" }, { key: "X-Robots-Tag", value: "noindex, nofollow" }] })),
    ];
  },
};

export default nextConfig;
