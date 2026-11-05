const API_PREFIX = "/api/v1";
import { isDemoMode } from "./demo/mode";
const DEFAULT_SITE_URL = "https://kingsgambit2700.in";

export class EnvironmentConfigurationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "EnvironmentConfigurationError";
  }
}

function validatedOrigin(name: string, value: string | undefined): string | null {
  if (!value) return null;
  if (value !== value.trim()) throw new EnvironmentConfigurationError(`${name} must not contain surrounding whitespace.`);
  if (value.endsWith("/")) throw new EnvironmentConfigurationError(`${name} must not have a trailing slash.`);

  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    throw new EnvironmentConfigurationError(`${name} must be an absolute HTTP(S) URL.`);
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    throw new EnvironmentConfigurationError(`${name} must use HTTP or HTTPS.`);
  }
  if (parsed.username || parsed.password || parsed.search || parsed.hash || parsed.pathname !== "/") {
    throw new EnvironmentConfigurationError(`${name} must contain only an origin.`);
  }
  return parsed.origin;
}

export function getSiteURL() {
  return validatedOrigin("NEXT_PUBLIC_SITE_URL", process.env.NEXT_PUBLIC_SITE_URL) || DEFAULT_SITE_URL;
}

export function getAPIBaseURL() {
  if (isDemoMode()) return "http://local-demo.invalid/api/v1";
  const serverOrigin = typeof window === "undefined"
    ? validatedOrigin("API_BASE_URL", process.env.API_BASE_URL)
    : null;
  const publicOrigin = validatedOrigin("NEXT_PUBLIC_API_BASE_URL", process.env.NEXT_PUBLIC_API_BASE_URL);
  const origin = serverOrigin || publicOrigin;
  return origin ? `${origin}${API_PREFIX}` : null;
}
