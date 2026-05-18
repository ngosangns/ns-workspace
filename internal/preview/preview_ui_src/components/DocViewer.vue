<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch, inject, type Ref } from "vue";
import { decorateInternalDocNavigation, type SpecDocument, type InternalSpecTarget } from "../js/internal-links.js";
import { destroyDiagramsIn, renderDiagramsIn } from "../js/diagrams.js";

interface Props {
  spec: SpecDocument | null;
  theme: "light" | "dark";
}

const props = defineProps<Props>();
const specs = inject<Ref<SpecDocument[]>>("specs");
const selectSpec = inject<(id: string, showSpecTab?: boolean) => Promise<void>>("selectSpec");
const specContent = ref<HTMLElement | null>(null);
const markdownViewerConstructor = ref<ToastMarkdownViewerConstructor | null>(null);
let renderToken = 0;

interface MetadataRow {
  key: string;
  value: string;
  href?: string;
}

const htmlDocSanitizeConfig = {
  ADD_TAGS: ["doc-meta", "doc-title", "doc-description", "doc-relation", "doc-diagram", "doc-graph"],
  ADD_ATTR: ["status", "compliance", "priority", "version", "tone", "type", "target", "href", "language"],
  FORBID_TAGS: ["script", "style"],
  FORBID_ATTR: ["style", "onclick", "onload", "onerror", "data-reactroot"],
};

function handleInternalLinkNavigation(target: InternalSpecTarget): void {
  if (selectSpec) {
    selectSpec(target.specId, true);
  }
}

async function renderSpecDocumentContent(root: HTMLElement, spec: SpecDocument) {
  const language = spec.language || languageFromPath(spec.path || "");
  destroyDiagramsIn(root);
  root.dataset.sourcePath = spec.path || spec.id || "";
  if (language === "markdown") {
    root.className = "markdown card border-base-300 bg-base-100 mx-auto max-w-5xl border p-6";
    await renderMarkdownPreview(root, spec.raw || "", props.theme);
    await renderDiagramsIn(root, props.theme, spec.id || spec.path || "markdown");
    decorateInternalDocNavigation(root, spec, specs?.value || [], handleInternalLinkNavigation);
  } else if (language === "html") {
    root.className = "markdown html-doc card border-base-300 bg-base-100 mx-auto max-w-5xl border p-6";
    renderHTMLPreview(root, spec.raw || "");
    await renderDiagramsIn(root, props.theme, spec.id || spec.path || "html");
    decorateInternalDocNavigation(root, spec, specs?.value || [], handleInternalLinkNavigation);
  } else {
    root.className = "card border-base-300 bg-base-100 mx-auto max-w-5xl border p-6";
    root.innerHTML = renderCodePreview(spec.raw || "", language);
  }
}

async function renderMarkdownPreview(root: HTMLElement, raw: string, theme: "light" | "dark") {
  const metadata = renderableMarkdownMetadata(raw);
  root.innerHTML = `${metadata.html}<div class="markdown-wysiwyg-host markdown-toast-viewer" data-markdown-viewer></div>`;
  const viewerHost = root.querySelector<HTMLElement>("[data-markdown-viewer]");
  if (!viewerHost) return;

  viewerHost.innerHTML = '<p class="text-base-content/60">Loading Markdown preview...</p>';
  try {
    const Viewer = await loadToastMarkdownViewer();
    viewerHost.innerHTML = "";
    new Viewer({
      el: viewerHost,
      height: "auto",
      initialValue: metadata.body || "No content.",
      theme: theme === "dark" ? "dark" : "default",
      usageStatistics: false,
    });
    viewerHost.innerHTML = DOMPurify.sanitize(viewerHost.innerHTML);
  } catch (error) {
    // Keep docs readable when the remote Markdown viewer CDN is unavailable.
    console.error(error);
    viewerHost.innerHTML = renderCodePreview(metadata.body || "No content.", "markdown");
  }
}

async function loadToastMarkdownViewer(): Promise<ToastMarkdownViewerConstructor> {
  if (markdownViewerConstructor.value) return markdownViewerConstructor.value;
  const moduleURL = "https://esm.sh/@toast-ui/editor@3.2.2/dist/toastui-editor-viewer?bundle&target=es2022";
  const viewerModule = (await import(moduleURL)) as { default?: ToastMarkdownViewerConstructor; Viewer?: ToastMarkdownViewerConstructor };
  const Viewer = viewerModule.default || viewerModule.Viewer;
  if (!Viewer) {
    throw new Error("TOAST UI Viewer failed to load.");
  }
  markdownViewerConstructor.value = Viewer;
  return Viewer;
}

function renderHTMLPreview(root: HTMLElement, raw: string) {
  root.innerHTML = DOMPurify.sanitize(raw || "<p>No content.</p>", htmlDocSanitizeConfig);
  const meta = root.querySelector("doc-meta");
  if (!meta) return;

  const metadata = document.createElement("div");
  const rows = htmlMetadataRows(meta);
  metadata.innerHTML = rows.length ? renderMetadataTable(rows) : "";
  meta.replaceWith(...metadata.childNodes);
}

function languageFromPath(path: string): string {
  const ext = path.split(".").pop()?.toLowerCase() || "";
  if (ext === "md" || ext === "markdown") return "markdown";
  if (ext === "html" || ext === "htm") return "html";
  return "text";
}

function renderCodePreview(raw: string, language: string): string {
  return `<pre class="bg-base-300 rounded p-4 overflow-auto"><code>${escapeHTML(raw)}</code></pre>`;
}

function escapeHTML(str: string): string {
  return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#039;");
}

function renderableMarkdownMetadata(raw: string): { html: string; body: string } {
  const lines = String(raw || "").split("\n");
  const rows: MetadataRow[] = [];
  let body = raw;
  if (lines[0]?.trim() !== "---") {
    const section = markdownBodyMetadata(body);
    return section.rows.length ? { html: renderMetadataTable(section.rows), body: section.body } : { html: "", body: raw };
  }

  const end = lines.slice(1).findIndex((line) => line.trim() === "---");
  if (end >= 0) {
    const metadataLines = lines.slice(1, end + 1);
    rows.push(...markdownMetadataRows(metadataLines));
    body = lines.slice(end + 2).join("\n");
  }

  const section = markdownBodyMetadata(body);
  rows.push(...section.rows);
  body = section.body;
  return {
    html: rows.length ? renderMetadataTable(rows) : "",
    body,
  };
}

function markdownMetadataRows(lines: string[]): MetadataRow[] {
  const rows: MetadataRow[] = [];
  let current: MetadataRow | null = null;
  lines.forEach((line) => {
    const keyValue = line.match(/^([A-Za-z0-9_.-]+):\s*(.*)$/);
    if (keyValue) {
      current = { key: keyValue[1], value: keyValue[2].trim() };
      rows.push(current);
      return;
    }

    const listItem = line.match(/^\s*-\s+(.*)$/);
    if (listItem && current) {
      current.value = appendMetadataValue(current.value, listItem[1].trim());
      return;
    }

    const continuation = line.trim();
    if (continuation && current) {
      current.value = appendMetadataValue(current.value, continuation);
    }
  });
  return rows.filter((row) => row.key);
}

function markdownBodyMetadata(raw: string): { rows: MetadataRow[]; body: string } {
  const lines = String(raw || "").split("\n");
  const start = lines.findIndex((line) => /^##\s+Meta\s*$/i.test(line.trim()));
  if (start < 0) return { rows: [], body: raw };

  const relativeEnd = lines.slice(start + 1).findIndex((line) => /^##\s+\S/.test(line.trim()));
  const end = relativeEnd < 0 ? lines.length : start + 1 + relativeEnd;
  const metadataLines = lines.slice(start + 1, end);
  const rows = markdownBodyMetadataRows(metadataLines);
  if (!rows.length) return { rows: [], body: raw };

  const body = [...lines.slice(0, start), ...lines.slice(end)].join("\n").replace(/\n{3,}/g, "\n\n");
  return { rows, body };
}

function markdownBodyMetadataRows(lines: string[]): MetadataRow[] {
  const rows: MetadataRow[] = [];
  let current: MetadataRow | null = null;
  lines.forEach((line) => {
    const bullet = line.match(/^\s*-\s+(?:\*\*)?(.+?)(?:\*\*)?:\s*(.*)$/);
    if (bullet) {
      current = { key: cleanMetadataScalar(bullet[1]), value: bullet[2].trim() };
      rows.push(current);
      return;
    }

    const listItem = line.match(/^\s*-\s+(.*)$/);
    if (listItem && current) {
      current.value = appendMetadataValue(current.value, listItem[1].trim());
      return;
    }

    const continuation = line.trim();
    if (continuation && current) {
      current.value = appendMetadataValue(current.value, continuation);
    }
  });
  return rows.filter((row) => row.key);
}

function htmlMetadataRows(meta: Element): MetadataRow[] {
  const rows: MetadataRow[] = [];
  ["status", "compliance", "priority", "version", "tone"].forEach((key) => {
    const value = meta.getAttribute(key);
    if (value) rows.push({ key, value });
  });

  const title = meta.querySelector("doc-title")?.textContent?.trim();
  const description = meta.querySelector("doc-description")?.textContent?.trim();
  if (title) rows.push({ key: "title", value: title });
  if (description) rows.push({ key: "description", value: description });

  meta.querySelectorAll("a[href]").forEach((link) => {
    const target = link.getAttribute("href") || "";
    const label = link.textContent?.trim() || target;
    if (target) rows.push({ key: "link", value: label, href: target });
  });
  meta.querySelectorAll("doc-relation[target]").forEach((relation) => {
    const target = relation.getAttribute("target") || "";
    const type = relation.getAttribute("type") || "related";
    if (target) rows.push({ key: `relation.${type}`, value: target, href: target });
  });
  return rows;
}

function renderMetadataTable(rows: MetadataRow[]): string {
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

function cleanMetadataScalar(value: string): string {
  const trimmed = String(value || "").trim();
  if (trimmed.length >= 2 && ((trimmed.startsWith('"') && trimmed.endsWith('"')) || (trimmed.startsWith("'") && trimmed.endsWith("'")))) {
    return trimmed.slice(1, -1);
  }
  return trimmed;
}

function appendMetadataValue(value: string, next: string): string {
  if (!next) return value;
  return value ? `${value}, ${next}` : next;
}

async function renderCurrentSpec() {
  const spec = props.spec;
  const root = specContent.value;
  if (!spec || !root) return;
  const token = ++renderToken;
  await renderSpecDocumentContent(root, spec);
  if (token !== renderToken) return;
}

onMounted(() => {
  void renderCurrentSpec();
});

onUnmounted(() => {
  if (specContent.value) {
    destroyDiagramsIn(specContent.value);
  }
});

watch(
  () => [props.spec, props.theme] as const,
  () => {
    void renderCurrentSpec();
  },
  { flush: "post" },
);
</script>

<template>
  <article ref="specContent" class="markdown card border-base-300 bg-base-100 mx-auto max-w-5xl border p-6"></article>
</template>
