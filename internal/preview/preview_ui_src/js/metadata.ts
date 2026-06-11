import { escapeHTML } from "./code-preview.js";

export interface MetadataRow {
  key: string;
  value: string;
  href?: string;
}

export function renderMetadataTable(rows: MetadataRow[]): string {
  const body = groupMetadataRows(rows)
    .map((group) => `<tr><th>${escapeHTML(group.key)}</th><td>${renderMetadataGroupValue(group)}</td></tr>`)
    .join("");
  return `<table class="metadata-table"><thead><tr><th>Metadata</th><th>Value</th></tr></thead><tbody>${body}</tbody></table>\n`;
}

function groupMetadataRows(rows: MetadataRow[]): Array<{ key: string; rows: MetadataRow[] }> {
  const groups: Array<{ key: string; rows: MetadataRow[] }> = [];
  rows.forEach((row) => {
    const key = String(row.key || "").trim();
    if (!key) return;
    const existing = groups.find((group) => group.key === key);
    if (existing) {
      existing.rows.push(row);
      return;
    }
    groups.push({ key, rows: [row] });
  });
  return groups;
}

function renderMetadataGroupValue(group: { key: string; rows: MetadataRow[] }): string {
  if (group.rows.length === 1) {
    const row = group.rows[0];
    return renderMetadataValue(row.value, row.key, row.href);
  }
  if (isMetadataReferenceKey(group.key) || group.rows.some((row) => row.href)) {
    const links = group.rows.flatMap((row) => {
      if (row.href) return [{ label: cleanMetadataScalar(row.value) || row.href, href: row.href }];
      return metadataReferenceLinks(row.value);
    });
    if (links.length) return renderMetadataLinkBadges(links);
  }
  return `<span class="metadata-badges">${group.rows
    .map((row) => cleanMetadataScalar(row.value))
    .filter(Boolean)
    .map((value) => `<span class="badge badge-ghost badge-sm">${escapeHTML(value)}</span>`)
    .join("")}</span>`;
}

function renderMetadataValue(raw: string, key = "", href = ""): string {
  if (href) {
    return renderMetadataLinkBadges([{ label: cleanMetadataScalar(raw) || href, href }]);
  }
  if (isMetadataReferenceKey(key)) {
    const links = metadataReferenceLinks(raw);
    if (links.length) return renderMetadataLinkBadges(links);
  }
  const values = metadataArrayValues(raw);
  if (values.length) {
    return `<span class="metadata-badges">${values.map((value) => `<span class="badge badge-ghost badge-sm">${escapeHTML(value)}</span>`).join("")}</span>`;
  }
  return escapeHTML(cleanMetadataScalar(raw));
}

function isMetadataReferenceKey(key: string): boolean {
  const normalized = String(key || "")
    .trim()
    .toLowerCase();
  return (
    ["link", "links", "related", "relations", "refs", "references", "docs_refs", "docs-refs"].includes(normalized) ||
    normalized.startsWith("relation.")
  );
}

function renderMetadataLinkBadges(links: Array<{ label: string; href: string }>): string {
  return `<span class="metadata-badges metadata-link-badges">${links
    .filter((link) => link.href)
    .map(
      (link) =>
        `<a class="badge badge-ghost badge-sm" href="${escapeHTML(link.href)}" title="${escapeHTML(link.href)}">${escapeHTML(link.label || link.href)}</a>`,
    )
    .join("")}</span>`;
}

function metadataReferenceLinks(raw: string): Array<{ label: string; href: string }> {
  const value = String(raw || "").trim();
  if (!value) return [];
  const markdownLinks = [...value.matchAll(/\[([^\]]+)\]\(([^)]+)\)/g)].map((match) => ({
    label: cleanMetadataScalar(match[1]),
    href: cleanMetadataScalar(match[2]),
  }));
  if (markdownLinks.length) return markdownLinks;
  return metadataListValues(value).map((item) => ({ label: item, href: item }));
}

function metadataArrayValues(raw: string): string[] {
  const value = String(raw || "").trim();
  if (!value) return [];
  if (value.startsWith("[") && value.endsWith("]")) {
    try {
      const parsed = JSON.parse(value) as unknown;
      if (Array.isArray(parsed)) {
        return parsed.map((item) => cleanMetadataScalar(String(item))).filter(Boolean);
      }
    } catch {
      return value.slice(1, -1).split(",").map(cleanMetadataScalar).filter(Boolean);
    }
  }
  return [];
}

function metadataListValues(raw: string): string[] {
  const arrayValues = metadataArrayValues(raw);
  if (arrayValues.length) return arrayValues;
  return String(raw || "")
    .split(",")
    .map(cleanMetadataScalar)
    .filter(Boolean);
}

export function cleanMetadataScalar(value: string): string {
  const trimmed = String(value || "").trim();
  if (trimmed.length >= 2 && ((trimmed.startsWith('"') && trimmed.endsWith('"')) || (trimmed.startsWith("'") && trimmed.endsWith("'")))) {
    return trimmed.slice(1, -1);
  }
  return trimmed;
}

export function appendMetadataValue(value: string, next: string): string {
  if (!next) return value;
  return value ? `${value}, ${next}` : next;
}
