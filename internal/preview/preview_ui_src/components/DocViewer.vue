<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch, inject, type Ref } from "vue";
import { decorateInternalDocNavigation, type SpecDocument, type InternalSpecTarget } from "../js/internal-links.js";
import { destroyDiagramsIn, renderDiagramsIn } from "../js/diagrams.js";
import { renderCodePreview } from "../js/code-preview.js";
import { renderHTMLPreview } from "../js/html-doc.js";
import { renderMarkdownPreview } from "../js/markdown.js";

interface Props {
  spec: SpecDocument | null;
  theme: "light" | "dark";
}

const props = defineProps<Props>();
const specs = inject<Ref<SpecDocument[]>>("specs");
const selectSpec = inject<(id: string, showSpecTab?: boolean) => Promise<void>>("selectSpec");
const specContent = ref<HTMLElement | null>(null);
let renderToken = 0;

function handleInternalLinkNavigation(target: InternalSpecTarget): void {
  if (selectSpec) {
    selectSpec(target.specId, true);
  }
}

async function renderSpecDocumentContent(root: HTMLElement, spec: SpecDocument): Promise<void> {
  const language = spec.language || languageFromPath(spec.path || "");
  destroyDiagramsIn(root);
  root.dataset.sourcePath = spec.path || spec.id || "";

  if (language === "markdown") {
    root.className = "markdown card border-base-300 bg-base-100 mx-auto max-w-5xl border p-6";
    await renderMarkdownPreview(root, spec.raw || "", props.theme);
    await decorateRenderedDoc(root, spec, "markdown");
    return;
  }

  if (language === "html") {
    root.className = "markdown html-doc card border-base-300 bg-base-100 mx-auto max-w-5xl border p-6";
    await renderHTMLPreview(root, spec.raw || "");
    await decorateRenderedDoc(root, spec, "html");
    return;
  }

  root.className = "card border-base-300 bg-base-100 mx-auto max-w-5xl border p-6";
  root.innerHTML = renderCodePreview(spec.raw || "", language);
}

async function decorateRenderedDoc(root: HTMLElement, spec: SpecDocument, fallbackKey: string): Promise<void> {
  await renderDiagramsIn(root, props.theme, spec.id || spec.path || fallbackKey);
  decorateInternalDocNavigation(root, spec, specs?.value || [], handleInternalLinkNavigation);
}

function languageFromPath(path: string): string {
  const ext = path.split(".").pop()?.toLowerCase() || "";
  if (ext === "md" || ext === "markdown") return "markdown";
  if (ext === "html" || ext === "htm") return "html";
  return "text";
}

async function renderCurrentSpec(): Promise<void> {
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
