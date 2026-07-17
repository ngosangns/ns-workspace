import type { CatalogSkill } from "../../api";

export function formatInstalls(n?: number): string {
  if (n == null || n <= 0) return "";
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

export function catalogKey(hit: CatalogSkill): string {
  return hit.id || `${hit.source}/${hit.skillId}`;
}

export function sourceLabel(source: string): string {
  switch (source) {
    case "installed":
      return "Installed";
    case "overlay":
      return "Custom";
    default:
      return "Default";
  }
}

export function sourceKind(source: string): "accent" | "ok" | "muted" {
  if (source === "overlay") return "accent";
  if (source === "installed") return "ok";
  return "muted";
}
