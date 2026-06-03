import { renderCodePreview } from "./code-preview.js";
import { appendMetadataValue, cleanMetadataScalar, renderMetadataTable, type MetadataRow } from "./metadata.js";

let markdownViewerConstructor: ToastMarkdownViewerConstructor | null = null;

export async function renderMarkdownPreview(root: HTMLElement, raw: string, theme: "light" | "dark"): Promise<void> {
  const metadata = renderableMarkdownMetadata(raw);
  root.innerHTML = `${metadata.html}<div class="markdown-wysiwyg-host markdown-toast-viewer" data-markdown-viewer></div>`;
  const viewerHost = root.querySelector<HTMLElement>("[data-markdown-viewer]");
  if (!viewerHost) return;

  viewerHost.innerHTML = '<p class="text-base-content/60 toast-markdown-loading">Loading Markdown preview...</p>';
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
    // Keep documents readable when the remote Markdown viewer CDN is unavailable.
    console.error(error);
    viewerHost.innerHTML = renderCodePreview(metadata.body || "No content.", "markdown");
  }
}

async function loadToastMarkdownViewer(): Promise<ToastMarkdownViewerConstructor> {
  if (markdownViewerConstructor) return markdownViewerConstructor;
  const moduleURL = "https://esm.sh/@toast-ui/editor@3.2.2/dist/toastui-editor-viewer?bundle&target=es2022";
  const viewerModule = (await import(moduleURL)) as { default?: ToastMarkdownViewerConstructor; Viewer?: ToastMarkdownViewerConstructor };
  const Viewer = viewerModule.default || viewerModule.Viewer;
  if (!Viewer) throw new Error("TOAST UI Viewer failed to load.");
  markdownViewerConstructor = Viewer;
  return Viewer;
}

export function renderableMarkdownMetadata(raw: string): { html: string; body: string } {
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

export function markdownMetadataRows(lines: string[]): MetadataRow[] {
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

export function markdownBodyMetadata(raw: string): { rows: MetadataRow[]; body: string } {
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
