import { marked } from "marked";

/**
 * Resolve document body HTML for the preview SPA.
 * The Go PreviewHandler returns raw Markdown with empty html; clients must
 * render Markdown. When html is already populated, prefer it.
 */
export function renderDocumentBody(doc: { html?: string; raw?: string }): string {
  const html = doc.html?.trim() ?? "";
  if (html) return html;
  const raw = doc.raw ?? "";
  if (!raw) return "";
  return marked.parse(raw, { breaks: false, gfm: true }) as string;
}
