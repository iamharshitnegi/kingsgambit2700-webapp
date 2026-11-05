export const apiVersion = process.env.NEXT_PUBLIC_SANITY_API_VERSION || "2026-01-06";
export const projectId = process.env.NEXT_PUBLIC_SANITY_PROJECT_ID || "localdev";
export const dataset = process.env.NEXT_PUBLIC_SANITY_DATASET || "production";
const editorialFlag = process.env.EDITORIAL_CONTENT_ENABLED || "false";

if (editorialFlag !== "true" && editorialFlag !== "false") {
  throw new Error("EDITORIAL_CONTENT_ENABLED must be either true or false.");
}

export const editorialContentEnabled = editorialFlag === "true";
const hasSanityConfiguration = Boolean(
  process.env.NEXT_PUBLIC_SANITY_PROJECT_ID && process.env.NEXT_PUBLIC_SANITY_DATASET,
);

if (editorialContentEnabled && !hasSanityConfiguration) {
  throw new Error("Sanity project and dataset are required when editorial content is enabled.");
}

export const sanityConfigured = editorialContentEnabled && hasSanityConfiguration;
