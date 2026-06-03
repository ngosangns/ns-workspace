import { escapeHTML } from "./code-preview.js";
import { renderMetadataTable, type MetadataRow } from "./metadata.js";

export const htmlMVPStylesheetURL = "https://cdn.jsdelivr.net/npm/mvp.css@1.17.3/mvp.css";

export const htmlDocSanitizeConfig = {
  ADD_TAGS: [
    "doc-meta",
    "doc-title",
    "doc-description",
    "doc-relation",
    "doc-callout",
    "doc-code",
    "doc-diagram",
    "doc-section",
    "doc-grid",
    "doc-card",
    "doc-steps",
    "doc-step",
    "doc-flow",
    "doc-flow-step",
    "doc-graph",
    "doc-metrics",
    "doc-metric",
  ],
  ADD_ATTR: [
    "status",
    "compliance",
    "priority",
    "version",
    "tone",
    "type",
    "target",
    "href",
    "language",
    "title",
    "columns",
    "value",
    "label",
    "caption",
  ],
  FORBID_TAGS: ["script", "style"],
  FORBID_ATTR: ["style", "onclick", "onload", "onerror", "data-reactroot"],
};

let htmlMVPStylesheetPromise: Promise<void> | null = null;

export async function renderHTMLPreview(root: HTMLElement, raw: string): Promise<void> {
  await ensureHTMLMVPStylesheet();
  root.innerHTML = DOMPurify.sanitize(raw || "<p>No content.</p>", htmlDocSanitizeConfig);
  normalizeHTMLDocTags(root);
}

export async function ensureHTMLMVPStylesheet(): Promise<void> {
  if (document.querySelector("style[data-html-mvp-css]")) return;
  if (!htmlMVPStylesheetPromise) {
    htmlMVPStylesheetPromise = fetch(htmlMVPStylesheetURL)
      .then((response) => {
        if (!response.ok) throw new Error(`MVP.css request failed with ${response.status}`);
        return response.text();
      })
      .then((css) => {
        const style = document.createElement("style");
        style.dataset.htmlMvpCss = "yes";
        style.textContent = scopeMVPStylesheet(css);
        const appStylesheet = document.querySelector<HTMLLinkElement>('link[href="/style.css"]');
        document.head.insertBefore(style, appStylesheet || null);
      })
      .catch(() => {
        htmlMVPStylesheetPromise = null;
      });
  }
  await htmlMVPStylesheetPromise;
}

export function scopeMVPStylesheet(css: string): string {
  try {
    const sheet = new CSSStyleSheet();
    sheet.replaceSync(css);
    return [...sheet.cssRules].map(scopeMVPRule).join("\n");
  } catch {
    return "";
  }
}

function scopeMVPRule(rule: CSSRule): string {
  if (rule instanceof CSSStyleRule) {
    const selectors = rule.selectorText
      .split(",")
      .map((selector) => scopeMVPSelector(selector.trim()))
      .filter(Boolean)
      .join(", ");
    return selectors ? `${selectors}{${rule.style.cssText}}` : "";
  }
  if (rule instanceof CSSMediaRule) {
    return `@media ${rule.conditionText}{${[...rule.cssRules].map(scopeMVPRule).join("\n")}}`;
  }
  if (rule instanceof CSSSupportsRule) {
    return `@supports ${rule.conditionText}{${[...rule.cssRules].map(scopeMVPRule).join("\n")}}`;
  }
  return rule.cssText;
}

function scopeMVPSelector(selector: string): string {
  if (!selector || selector.startsWith("@")) return selector;
  if (selector === ":root" || selector === "html" || selector === "body") return ".html-doc";
  if (selector.startsWith(":root ") || selector.startsWith("html ") || selector.startsWith("body ")) {
    return selector.replace(/^(?::root|html|body)/, ".html-doc");
  }
  return `.html-doc ${selector}`;
}

export function normalizeHTMLDocTags(root: HTMLElement): void {
  root.querySelectorAll("doc-meta").forEach((node) => {
    const metadata = document.createElement("div");
    const rows = htmlMetadataRows(node);
    metadata.innerHTML = rows.length ? renderMetadataTable(rows) : "";
    node.replaceWith(...metadata.childNodes);
  });
  root.querySelectorAll("doc-title").forEach((node) => replaceDocElement(node, "h1", "doc-title"));
  root.querySelectorAll("doc-description").forEach((node) => replaceDocElement(node, "p", "doc-description"));
  root.querySelectorAll("doc-callout").forEach((node) => {
    const tone = sanitizeClassToken(node.getAttribute("tone") || "info");
    replaceDocElement(node, "aside", `doc-callout doc-callout-${tone}`);
  });
  root.querySelectorAll("doc-relation").forEach((node) => {
    const relation = document.createElement("a");
    const target = node.getAttribute("target") || node.getAttribute("href") || "";
    const type = node.getAttribute("type") || "related";
    const typeClass = sanitizeClassToken(type);
    relation.className = `doc-relation doc-relation-${typeClass}`;
    relation.href = target;
    relation.textContent = node.textContent?.trim() || target || type;
    node.replaceWith(relation);
  });
  root.querySelectorAll("doc-code").forEach((node) => {
    const language = normalizeDocCodeLanguage(node.getAttribute("language") || node.getAttribute("type") || "");
    const pre = document.createElement("pre");
    pre.className = "doc-code-block";
    pre.dataset.sourceLanguage = language;
    pre.innerHTML = `<code class="language-${escapeHTML(language)}">${escapeHTML(node.textContent || "")}</code>`;
    node.replaceWith(pre);
  });
  root.querySelectorAll("doc-diagram").forEach((node) => {
    const language = normalizeDocDiagramLanguage(node.getAttribute("language") || node.getAttribute("type") || "mermaid");
    node.replaceWith(createDocDiagramSource(node.textContent || "", language, "doc-diagram-source"));
  });
  root.querySelectorAll("doc-graph").forEach((node) => {
    const language = normalizeDocDiagramLanguage(node.getAttribute("language") || node.getAttribute("type") || "mermaid");
    node.replaceWith(createDocDiagramSource(node.textContent || "", language, "doc-diagram-source doc-graph-source"));
  });
  root.querySelectorAll("doc-section").forEach((node) => replaceDocContainer(node, "section", "doc-section", "h2"));
  root.querySelectorAll("doc-grid").forEach((node) => {
    const columns = sanitizeClassToken(node.getAttribute("columns") || "");
    replaceDocElement(node, "div", `doc-grid${columns ? ` doc-grid-${columns}` : ""}`);
  });
  root.querySelectorAll("doc-card").forEach((node) => replaceDocContainer(node, "article", "doc-card", "h3"));
  root.querySelectorAll("doc-steps").forEach((node) => replaceDocElement(node, "ol", "doc-steps"));
  root.querySelectorAll("doc-step").forEach((node) => replaceDocContainer(node, "li", "doc-step", "strong"));
  root.querySelectorAll("doc-flow").forEach((node) => replaceDocElement(node, "div", "doc-flow"));
  root.querySelectorAll("doc-flow-step").forEach((node) => replaceDocContainer(node, "div", "doc-flow-step", "strong"));
  root.querySelectorAll("doc-metrics").forEach((node) => replaceDocElement(node, "div", "doc-metrics"));
  root.querySelectorAll("doc-metric").forEach((node) => replaceDocMetric(node));
}

export function htmlMetadataRows(meta: Element): MetadataRow[] {
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

function replaceDocElement(node: Element, tagName: string, className: string): void {
  const replacement = document.createElement(tagName);
  replacement.className = className;
  replacement.innerHTML = node.innerHTML;
  node.replaceWith(replacement);
}

function replaceDocContainer(node: Element, tagName: string, className: string, titleTagName: string): void {
  const replacement = document.createElement(tagName);
  replacement.className = className;
  const title = node.getAttribute("title");
  if (title) {
    replacement.innerHTML = `<${titleTagName}>${escapeHTML(title)}</${titleTagName}>`;
  }
  moveNodeChildren(node, replacement);
  node.replaceWith(replacement);
}

function replaceDocMetric(node: Element): void {
  const metric = document.createElement("div");
  metric.className = "doc-metric";
  const value = node.getAttribute("value") || node.textContent?.trim() || "";
  const label = node.getAttribute("label") || "";
  const caption = node.getAttribute("caption") || "";
  metric.innerHTML = `
    <div class="doc-metric-value">${escapeHTML(value)}</div>
    ${label ? `<div class="doc-metric-label">${escapeHTML(label)}</div>` : ""}
    ${caption ? `<div class="doc-metric-caption">${escapeHTML(caption)}</div>` : ""}`;
  node.replaceWith(metric);
}

function createDocDiagramSource(source: string, language: string, className: string): HTMLElement {
  const pre = document.createElement("pre");
  pre.className = className;
  const code = document.createElement("code");
  code.className = `language-${language}`;
  code.dataset.sourceLanguage = language;
  code.textContent = source;
  pre.append(code);
  return pre;
}

function moveNodeChildren(from: Element, to: Element): void {
  while (from.firstChild) {
    to.append(from.firstChild);
  }
}

function normalizeDocCodeLanguage(language: string): string {
  return sanitizeClassToken(language || "text");
}

function normalizeDocDiagramLanguage(language: string): string {
  const normalized = sanitizeClassToken(language || "mermaid");
  return normalized === "c4model" ? "c4-model" : normalized;
}

function sanitizeClassToken(value: string): string {
  return String(value || "")
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, "-")
    .replace(/^-+|-+$/g, "");
}
