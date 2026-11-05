import createImageUrlBuilder from "@sanity/image-url";
import { dataset, projectId, sanityConfigured } from "../env";

const builder = sanityConfigured ? createImageUrlBuilder({ projectId, dataset }) : null;
type ImageSource = Parameters<ReturnType<typeof createImageUrlBuilder>["image"]>[0];

export function urlFor(source: ImageSource) {
  if (!builder) throw new Error("Sanity image service is not configured.");
  return builder.image(source);
}
