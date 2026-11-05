import type { CSSProperties } from "react";
import config from "@/design/brand.config.json";

export const brandConfig = config;
export const brandAccent = config.accents[config.defaultAccent as keyof typeof config.accents];

if (!brandAccent) throw new Error(`Unknown brand accent: ${config.defaultAccent}`);

const kebab = (key: string) => key.replace(/[A-Z]/g, (letter) => `-${letter.toLowerCase()}`);

/** Server-rendered custom properties keep every route on the same brand configuration. */
export function getBrandStyles(): CSSProperties {
  const tokens: Record<string, string | number> = {};
  for (const [name, value] of Object.entries({ ...config.colors, ...brandAccent })) {
    if (name !== "label") tokens[`--${kebab(name)}`] = value;
  }
  for (const name of ["display", "body", "mono"] as const) tokens[`--font-${name}`] = config.typography[name];
  tokens["--font-display-weight"] = config.typography.displayWeight;
  tokens["--font-body-weight"] = config.typography.bodyWeight;
  for (const [name, value] of Object.entries(config.typography.scale)) tokens[`--text-${name}`] = value;
  for (const [values, prefix] of [
    [config.spacing, "space"], [config.radius, "radius"], [config.layout, "layout"],
    [config.motion, "motion"], [config.shadow, "shadow"],
  ] as const) {
    for (const [name, value] of Object.entries(values)) tokens[`--${prefix}-${kebab(name)}`] = value;
  }
  return tokens as CSSProperties;
}
