import type { ProblemDetail } from "./types";
import { getAPIBaseURL } from "../env";
import { isDemoMode } from "../demo/mode";

const DEFAULT_REVALIDATE_SECONDS = 300;

let csrfToken: string | undefined;
let recoverSession: (() => Promise<boolean>) | undefined;
let recovering: Promise<boolean> | undefined;

export function setSessionRecovery(recover: () => Promise<boolean>) { recoverSession = recover; }

/** Retry only explicit authentication/CSRF rejections, before domain mutation. */
export async function authenticatedFetch(request: Request): Promise<Response> {
  const retry = request.clone();
  const response = await fetch(request, { cache: "no-store" });
  if (!recoverSession || /\/api\/v1\/(auth|public)\//.test(new URL(request.url).pathname)) return response;
  if (response.status !== 401 && response.status !== 403) return response;
  const problem = await response.clone().json().catch(() => null) as ProblemDetail | null;
  if (response.status !== 401 && problem?.code !== "invalid_csrf") return response;
  recovering ??= recoverSession().finally(() => { recovering = undefined; });
  if (!await recovering) return response;
  if (csrfToken && !["GET", "HEAD", "OPTIONS"].includes(retry.method)) retry.headers.set("X-CSRF-Token", csrfToken);
  return fetch(retry, { cache: "no-store" });
}

export class APIError extends Error {
  status: number;
  problem?: ProblemDetail;

  constructor(message: string, status: number, problem?: ProblemDetail) {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.problem = problem;
  }
}

export function setCSRFToken(token?: string) {
  csrfToken = token;
}

export function getCSRFToken() {
  return csrfToken;
}

export { getAPIBaseURL } from "../env";

function resolveAPIURL(path: string) {
  const baseURL = getAPIBaseURL();
  if (!baseURL) {
    throw new APIError(
      "The academy service is not configured for this environment.",
      503,
    );
  }

  if (!path.startsWith("/")) {
    throw new Error("API paths must begin with a forward slash.");
  }

  return `${baseURL}${path}`;
}

type NextRequestInit = RequestInit & {
  next?: { revalidate?: number; tags?: string[] };
};

type APIRequestOptions = {
  publicCache?: boolean;
  revalidate?: number;
};

export async function apiFetch<T>(
  path: string,
  init: NextRequestInit = {},
  options: APIRequestOptions = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");

  const method = (init.method ?? "GET").toUpperCase();
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  if (!["GET", "HEAD", "OPTIONS"].includes(method) && csrfToken) {
    headers.set("X-CSRF-Token", csrfToken);
  }

  const requestInit: NextRequestInit = {
    ...init,
    method,
    headers,
    credentials: "include",
  };

  if (options.publicCache) {
    requestInit.next = {
      revalidate: options.revalidate ?? DEFAULT_REVALIDATE_SECONDS,
    };
  } else {
    requestInit.cache = "no-store";
  }

  let response: Response;
  try {
    response = isDemoMode()
      ? await (await import("../demo/transport")).demoFetch(resolveAPIURL(path), requestInit)
      : options.publicCache || /^\/(auth|public)\//.test(path) ? await fetch(resolveAPIURL(path), requestInit)
      : await authenticatedFetch(new Request(resolveAPIURL(path), requestInit));
  } catch (error) {
    if (error instanceof APIError) throw error;
    throw new APIError("The academy service could not be reached.", 503);
  }

  if (response.status === 204) return undefined as T;

  const contentType = response.headers.get("content-type") ?? "";
  const body = contentType.includes("json")
    ? await response.json().catch(() => undefined)
    : await response.text().catch(() => undefined);

  if (!response.ok) {
    const problem =
      body && typeof body === "object" ? (body as ProblemDetail) : undefined;
    throw new APIError(
      problem?.detail || problem?.title || `Request failed (${response.status}).`,
      response.status,
      problem,
    );
  }

  return body as T;
}

export function getFieldErrors(error: unknown) {
  if (!(error instanceof APIError) || !error.problem?.errors) return {};

  return Object.fromEntries(
    error.problem.errors.map(({ field, message }) => [field, message]),
  );
}
