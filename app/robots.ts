import type { MetadataRoute } from "next";
import { getSiteURL } from "./lib/env";

const siteURL = getSiteURL();

export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: "*",
      allow: "/",
      disallow: [
        "/admin",
        "/api/",
        "/auth/",
        "/classroom/",
        "/coach",
        "/demo",
        "/forbidden",
        "/guardian",
        "/portal",
        "/sign-in",
        "/student-access",
        "/student",
        "/studio/",
      ],
    },
    sitemap: `${siteURL}/sitemap.xml`,
    host: siteURL,
  };
}
