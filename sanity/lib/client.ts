import { createClient, type SanityClient } from "next-sanity";
import { apiVersion, dataset, projectId, sanityConfigured } from "../env";

export const client: SanityClient | null = sanityConfigured
  ? createClient({ projectId, dataset, apiVersion, useCdn: true })
  : null;
