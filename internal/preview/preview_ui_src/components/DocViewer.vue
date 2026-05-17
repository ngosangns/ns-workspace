<script setup lang="ts">
import { onMounted, ref, watch, inject, type Ref } from "vue";
import { decorateInternalDocNavigation, type SpecDocument, type InternalSpecTarget } from "../js/internal-links.js";

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

function handleInternalLinkNavigation(target: InternalSpecTarget): void {
  if (selectSpec) {
    selectSpec(target.specId, true);
  }
}

async function renderSpecDocumentContent(root: HTMLElement, spec: SpecDocument) {
  const language = spec.language || languageFromPath(spec.path || "");
  root.dataset.sourcePath = spec.path || spec.id || "";
  if (language === "markdown") {
    root.className = "markdown card border-base-300 bg-base-100 mx-auto max-w-5xl border p-6";
    await renderMarkdownPreview(root, spec.raw || "", props.theme);
    decorateInternalDocNavigation(root, spec, specs?.value || [], handleInternalLinkNavigation);
  } else if (language === "html") {
    root.className = "markdown html-doc card border-base-300 bg-base-100 mx-auto max-w-5xl border p-6";
    renderHTMLPreview(root, spec.raw || "");
    decorateInternalDocNavigation(root, spec, specs?.value || [], handleInternalLinkNavigation);
  } else {
    root.className = "card border-base-300 bg-base-100 mx-auto max-w-5xl border p-6";
    root.innerHTML = renderCodePreview(spec.raw || "", language);
  }
}

async function renderMarkdownPreview(root: HTMLElement, raw: string, theme: "light" | "dark") {
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
      theme: theme === "dark" ? "dark" : "default",
      usageStatistics: false,
    });
    viewerHost.innerHTML = DOMPurify.sanitize(viewerHost.innerHTML);
  } catch (error) {
    // Keep docs readable when the remote Markdown viewer CDN is unavailable.
    console.error(error);
    viewerHost.innerHTML = renderCodePreview(raw || "No content.", "markdown");
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
  root.innerHTML = DOMPurify.sanitize(raw || "<p>No content.</p>");
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
