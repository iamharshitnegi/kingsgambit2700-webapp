import { fetchPolicyVersion, getPublicAPIClient, unwrapAPIResponse } from "./api/typed-client";
import { EnvironmentConfigurationError } from "./env";
import type {
  Coach,
  Curriculum,
  OrganizationSettings,
  Policy,
  PolicySummary,
  PricingCatalog,
  Program,
} from "./api/types";
import {
  fallbackCurriculum,
  fallbackOrganization,
  fallbackPolicies,
  fallbackPolicySummaries,
  fallbackPrograms,
} from "./fallback-content";

function displayPhone(value: string) {
  if (value === "+918679762608") return "+91 86797 62608";
  return value;
}

function unavailableFallback<T>(error: unknown, fallback: T): T {
  if (error instanceof EnvironmentConfigurationError) throw error;
  return fallback;
}

export async function getOrganization(): Promise<OrganizationSettings> {
  try {
    const result = await getPublicAPIClient().GET("/public/organization");
    const organization = unwrapAPIResponse(result);
    const whatsApp = organization.whatsApp || organization.phone || fallbackOrganization.whatsappNumber;
    return {
      name: organization.name,
      legalName: organization.legalName,
      contactEmail: organization.email || fallbackOrganization.contactEmail,
      whatsappNumber: whatsApp,
      whatsappDisplay: displayPhone(whatsApp),
      primaryTimeZone: fallbackOrganization.primaryTimeZone,
      siteUrl: organization.canonicalUrl || fallbackOrganization.siteUrl,
      instagramUrl: organization.socialLinks.instagram || fallbackOrganization.instagramUrl,
      facebookUrl: organization.socialLinks.facebook,
      youtubeUrl: organization.socialLinks.youtube,
    };
  } catch (error) {
    return unavailableFallback(error, fallbackOrganization);
  }
}

export async function getPrograms(): Promise<Program[]> {
  try {
    const result = await getPublicAPIClient().GET("/public/programs");
    const programs = unwrapAPIResponse(result).items;
    return programs.length ? programs : fallbackPrograms;
  } catch (error) {
    return unavailableFallback(error, fallbackPrograms);
  }
}

export async function getCoaches(): Promise<Coach[]> {
  try {
    const result = await getPublicAPIClient().GET("/public/coaches");
    return unwrapAPIResponse(result).items;
  } catch (error) {
    return unavailableFallback<Coach[]>(error, []);
  }
}

export async function getCurriculum(): Promise<Curriculum> {
  try {
    const result = await getPublicAPIClient().GET("/public/curriculum");
    const curriculum = unwrapAPIResponse(result);
    return curriculum.tracks.length ? curriculum : fallbackCurriculum;
  } catch (error) {
    return unavailableFallback(error, fallbackCurriculum);
  }
}

export async function getPricing(): Promise<PricingCatalog> {
  try {
    const result = await getPublicAPIClient().GET("/public/pricing");
    const pricing = unwrapAPIResponse(result);
    return {
      currency: pricing.currency,
      items: pricing.items.map((item) => ({ ...item, currency: pricing.currency })),
    };
  } catch (error) {
    return unavailableFallback(error, { currency: "INR", items: [] });
  }
}

export async function getPolicySummaries(): Promise<PolicySummary[]> {
  try {
    const result = await getPublicAPIClient().GET("/public/policies");
    const policies = unwrapAPIResponse(result).items;
    if (!policies.length) return fallbackPolicySummaries;
    return policies.map((policy) => ({
      slug: policy.slug,
      title: policy.title,
      summary: policy.purpose,
      version: policy.version,
      effectiveAt: policy.effectiveAt,
      reviewStatus: "published",
    }));
  } catch (error) {
    return unavailableFallback(error, fallbackPolicySummaries);
  }
}

export async function getPolicy(slug: string): Promise<Policy | null> {
  const fallback = fallbackPolicies[slug];
  try {
    const result = await getPublicAPIClient().GET("/public/policies/{slug}", {
      params: { path: { slug } },
    });
    const policy = unwrapAPIResponse(result);
    return {
      slug: policy.slug,
      title: policy.title,
      summary: policy.purpose,
      version: policy.version,
      effectiveAt: policy.effectiveAt,
      reviewStatus: "published",
      contentHtml: policy.contentHtml,
    };
  } catch (error) {
    return unavailableFallback(error, fallback || null);
  }
}

export async function getPolicyVersion(slug: string, version: string): Promise<Policy | null> {
  try {
    const policy = await fetchPolicyVersion(slug, version);
    return {
      slug: policy.slug,
      title: policy.title,
      summary: policy.purpose,
      version: policy.version,
      effectiveAt: policy.effectiveAt,
      reviewStatus: "published",
      contentHtml: policy.contentHtml,
    };
  } catch (error) {
    if (error instanceof EnvironmentConfigurationError) throw error;
    return null;
  }
}
