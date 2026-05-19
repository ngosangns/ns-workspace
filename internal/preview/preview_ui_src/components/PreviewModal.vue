<script setup lang="ts">
import { ref, watch, computed, inject, nextTick, onUnmounted, type Ref } from "vue";
import { decorateInternalDocNavigation, type SpecDocument, type InternalSpecTarget } from "../js/internal-links.js";
import Icon from "./Icon.vue";
import { destroyDiagramsIn, renderDiagramsIn } from "../js/diagrams.js";

interface PreviewSource {
  type: "doc" | "file";
  raw: string;
  language: string;
  path: string;
  line: number;
  spec?: SpecDocument;
}

interface Props {
  source: PreviewSource | null;
  showRaw: boolean;
}

const props = defineProps<Props>();
const emit = defineEmits<{
  (e: "close"): void;
  (e: "toggleRaw"): void;
}>();

const specs = inject<Ref<SpecDocument[]>>("specs");
const selectSpec = inject<(id: string, showSpecTab?: boolean) => Promise<void>>("selectSpec");
const theme = inject<Ref<"light" | "dark">>("theme", ref("light"));
const previewDialogBody = ref<HTMLElement | null>(null);
const markdownViewerConstructor = ref<ToastMarkdownViewerConstructor | null>(null);
const htmlMVPStylesheetURL = "https://cdn.jsdelivr.net/npm/mvp.css@1.17.2/mvp.css";
let htmlMVPStylesheetPromise: Promise<void> | null = null;
let renderToken = 0;

const dialogTitle = computed(() => {
  if (!props.source) return "Preview";
  const spec = props.source.spec;
  if (spec) return spec.title || "Doc preview";
  const fileName = props.source.path.split("/").pop() || "File preview";
  return props.source.line ? `${fileName}:${props.source.line}` : fileName;
});

const dialogPath = computed(() => {
  if (!props.source) return "";
  return props.source.path;
});

const isOpen = computed(() => props.source !== null);

function handleInternalLinkNavigation(target: InternalSpecTarget): void {
  if (selectSpec) {
    selectSpec(target.specId, true);
  }
  emit("close");
}

function escapeHTML(str: string): string {
  return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#039;");
}

function renderCodePreview(raw: string, language: string): string {
  return `<pre class="code-preview bg-base-300 rounded p-4 overflow-auto"><code>${escapeHTML(raw)}</code></pre>`;
}

function renderPreviewContentCard(content: string, extraClass = ""): string {
  const className = ["preview-content-card", extraClass].filter(Boolean).join(" ");
  return `<div class="${className}" data-preview-content>${content}</div>`;
}

function renderPreviewMetadata(source: PreviewSource): string {
  const rows: Array<{ key: string; value: string }> = [];
  const add = (key: string, value: unknown) => {
    const text = String(value || "").trim();
    if (text) rows.push({ key, value: text });
  };

  if (source.type === "doc" && source.spec) {
    add("title", source.spec.title);
    add("path", source.spec.path || source.path);
    add("language", source.spec.language || source.language);
    add("status", source.spec.status);
    add("compliance", source.spec.compliance);
    add("version", source.spec.version);
    add("priority", source.spec.priority);
    add("description", source.spec.description);
  } else {
    add("type", source.type);
    add("path", source.path);
    add("language", source.language);
    add("line", source.line ? String(source.line) : "");
  }

  if (!rows.length) return "";
  return `<table class="metadata-table preview-metadata"><thead><tr><th>Metadata</th><th>Value</th></tr></thead><tbody>${rows
    .map((row) => `<tr><th>${escapeHTML(row.key)}</th><td>${escapeHTML(row.value)}</td></tr>`)
    .join("")}</tbody></table>`;
}

async function renderPreviewSource() {
  const root = previewDialogBody.value;
  const source = props.source;
  if (!root || !source) return;
  const token = ++renderToken;
  destroyDiagramsIn(root);
  root.dataset.sourcePath = source.path;
  const metadata = renderPreviewMetadata(source);

  if (props.showRaw || source.type !== "doc" || !source.spec) {
    root.className = "preview-modal-body";
    root.innerHTML = renderPreviewContentCard(`${metadata}${renderCodePreview(source.raw || "", source.language || "text")}`);
    decorateCodePreviewLines(root);
    scrollPreviewToLine(root, source.line);
    return;
  }

  const spec = source.spec;
  const language = spec.language || languageFromPath(spec.path || source.path);
  if (language === "markdown") {
    root.className = "preview-modal-body markdown";
    root.innerHTML = renderPreviewContentCard("");
    const contentCard = root.querySelector<HTMLElement>("[data-preview-content]");
    if (!contentCard) return;
    await renderMarkdownPreview(contentCard, spec.raw || source.raw || "", theme.value);
    if (token !== renderToken) return;
    await renderDiagramsIn(contentCard, theme.value, spec.id || source.path || "preview-markdown");
    if (token !== renderToken) return;
    contentCard.insertAdjacentHTML("afterbegin", metadata);
    decorateInternalDocNavigation(root, spec, specs?.value || [], handleInternalLinkNavigation);
    return;
  }

  if (language === "html") {
    root.className = "preview-modal-body markdown";
    root.innerHTML = renderPreviewContentCard("", "html-doc");
    const contentCard = root.querySelector<HTMLElement>("[data-preview-content]");
    if (!contentCard) return;
    await renderHTMLPreview(contentCard, spec.raw || source.raw || "");
    if (token !== renderToken) return;
    await renderDiagramsIn(contentCard, theme.value, spec.id || source.path || "preview-html");
    if (token !== renderToken) return;
    contentCard.insertAdjacentHTML("afterbegin", metadata);
    decorateInternalDocNavigation(root, spec, specs?.value || [], handleInternalLinkNavigation);
    return;
  }

  root.className = "preview-modal-body";
  root.innerHTML = renderPreviewContentCard(`${metadata}${renderCodePreview(spec.raw || source.raw || "", language)}`);
  decorateCodePreviewLines(root);
  scrollPreviewToLine(root, source.line);
}

function decorateCodePreviewLines(root: HTMLElement): void {
  root.querySelectorAll<HTMLElement>("pre code").forEach((code) => {
    if (code.dataset.lines === "yes") return;
    const lines = code.textContent?.split("\n") || [];
    code.innerHTML = lines
      .map(
        (line, index) =>
          `<span class="code-line" data-line="${index + 1}"><span class="code-line-number">${index + 1}</span><span class="code-line-content">${escapeHTML(line || " ")}</span></span>`,
      )
      .join("");
    code.dataset.lines = "yes";
  });
}

function scrollPreviewToLine(root: HTMLElement, line: number): void {
  if (!line) return;
  window.setTimeout(() => {
    const target = root.querySelector<HTMLElement>(`[data-line="${line}"]`);
    if (!target) return;
    target.classList.add("code-line-target");
    target.scrollIntoView({ block: "center" });
  }, 40);
}

async function renderMarkdownPreview(root: HTMLElement, raw: string, currentTheme: "light" | "dark") {
  root.innerHTML = '<div class="markdown-wysiwyg-host markdown-toast-viewer" data-markdown-viewer></div>';
  const viewerHost = root.querySelector<HTMLElement>("[data-markdown-viewer]");
  if (!viewerHost) return;

  viewerHost.innerHTML = '<p class="text-base-content/60">Loading Markdown preview...</p>';
  try {
    const Viewer = await loadToastMarkdownViewer();
    viewerHost.innerHTML = "";
    new Viewer({
      el: viewerHost,
      height: "auto",
      initialValue: raw || "No content.",
      theme: currentTheme === "dark" ? "dark" : "default",
      usageStatistics: false,
    });
    viewerHost.innerHTML = DOMPurify.sanitize(viewerHost.innerHTML);
  } catch (error) {
    console.error(error);
    viewerHost.innerHTML = renderCodePreview(raw || "No content.", "markdown");
  }
}

async function loadToastMarkdownViewer(): Promise<ToastMarkdownViewerConstructor> {
  if (markdownViewerConstructor.value) return markdownViewerConstructor.value;
  const moduleURL = "https://esm.sh/@toast-ui/editor@3.2.2/dist/toastui-editor-viewer?bundle&target=es2022";
  const viewerModule = (await import(moduleURL)) as { default?: ToastMarkdownViewerConstructor; Viewer?: ToastMarkdownViewerConstructor };
  const Viewer = viewerModule.default || viewerModule.Viewer;
  if (!Viewer) throw new Error("TOAST UI Viewer failed to load.");
  markdownViewerConstructor.value = Viewer;
  return Viewer;
}

async function renderHTMLPreview(root: HTMLElement, raw: string) {
  await ensureHTMLMVPStylesheet();
  root.innerHTML = DOMPurify.sanitize(raw || "<p>No content.</p>", {
    ADD_TAGS: ["doc-meta", "doc-title", "doc-description", "doc-relation", "doc-diagram", "doc-graph"],
    ADD_ATTR: ["status", "compliance", "priority", "version", "tone", "type", "target", "href", "language"],
    FORBID_TAGS: ["script", "style"],
    FORBID_ATTR: ["style", "onclick", "onload", "onerror", "data-reactroot"],
  });
  normalizeHTMLDocTags(root);
}

async function ensureHTMLMVPStylesheet() {
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

function scopeMVPStylesheet(css: string): string {
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

function normalizeHTMLDocTags(root: HTMLElement) {
  root.querySelectorAll("doc-title").forEach((node) => replaceDocElement(node, "h1", "doc-title"));
  root.querySelectorAll("doc-description").forEach((node) => replaceDocElement(node, "p", "doc-description"));
}

function replaceDocElement(node: Element, tagName: string, className: string) {
  const replacement = document.createElement(tagName);
  replacement.className = className;
  replacement.innerHTML = node.innerHTML;
  node.replaceWith(replacement);
}

function languageFromPath(path: string): string {
  const ext = path.split(".").pop()?.toLowerCase() || "";
  if (ext === "md" || ext === "markdown") return "markdown";
  if (ext === "html" || ext === "htm") return "html";
  return "text";
}

watch(
  () => [props.source, props.showRaw, theme.value] as const,
  async () => {
    await nextTick();
    await renderPreviewSource();
  },
  { flush: "post" },
);

onUnmounted(() => {
  if (previewDialogBody.value) {
    destroyDiagramsIn(previewDialogBody.value);
  }
});
</script>

<template>
  <div v-if="isOpen" id="previewDialog" class="modal modal-open" role="dialog" aria-modal="true" aria-labelledby="previewDialogTitle">
    <div class="modal-box preview-modal border-base-300 bg-base-100 border p-0">
      <div class="preview-modal-header border-base-300 border-b">
        <div class="min-w-0">
          <h2 id="previewDialogTitle" class="truncate text-base font-semibold">{{ dialogTitle }}</h2>
          <p id="previewDialogPath" class="text-base-content/60 truncate text-xs">{{ dialogPath }}</p>
        </div>
        <div class="flex items-center gap-1">
          <button
            id="previewRawToggle"
            class="btn btn-ghost btn-sm"
            type="button"
            aria-label="View raw source"
            title="View raw source"
            @click="emit('toggleRaw')"
          >
            <Icon name="file-code" class="h-4 w-4" />
          </button>
          <button
            class="btn btn-ghost btn-sm"
            type="button"
            data-close-preview
            aria-label="Close preview"
            title="Close preview"
            @click="emit('close')"
          >
            <Icon name="x" class="h-4 w-4" />
          </button>
        </div>
      </div>
      <div id="previewDialogBody" ref="previewDialogBody" class="preview-modal-body"></div>
    </div>
    <button class="modal-backdrop" type="button" @click="emit('close')">close</button>
  </div>
</template>
