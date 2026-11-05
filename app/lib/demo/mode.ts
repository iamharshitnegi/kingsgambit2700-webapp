/** Explicit development-only switch. Never enables mock authentication in a production build. */
export function isDemoMode() {
  return process.env.NODE_ENV !== "production" && process.env.NEXT_PUBLIC_DEMO_MODE === "true";
}
