<script setup lang="ts">
import { ref, watch, computed, inject, nextTick, onUnmounted } from "vue";
import { decorateInternalDocNavigation, type SpecDocument, type InternalSpecTarget } from "../js/internal-links.js";
import Icon from "./Icon.vue";
import { destroyDiagramsIn, renderDiagramsIn } from "../js/diagrams.js";
import { decorateCodePreviewLines, escapeHTML, renderCodePreview, scrollPreviewToLine } from "../js/code-preview.js";
import { renderHTMLPreview } from "../js/html-doc.js";
import { renderMarkdownPreview } from "../js/markdown.js";
import { SpecsKey, SelectSpecKey, ThemeKey, type PreviewSource } from "../js/shared-types.js";
import { languageFromPath } from "../js/shared-utils.js";

interface Props {
  source: PreviewSource | null;
  showRaw: boolean;
}

const props = defineProps<Props>();
const emit = defineEmits<{
  (e: "close"): void;
  (e: "toggleRaw"): void;
}>();

const specs = inject(SpecsKey);
const selectSpec = inject(SelectSpecKey);
const theme = inject(ThemeKey, ref("light"));
const previewDialogBody = ref<HTMLElement | null>(null);
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

async function renderPreviewSource(): Promise<void> {
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
    await decorateRenderedDoc(contentCard, spec, source.path || "preview-markdown");
    if (token !== renderToken) return;
    contentCard.insertAdjacentHTML("afterbegin", metadata);
    return;
  }

  if (language === "html") {
    root.className = "preview-modal-body markdown";
    root.innerHTML = renderPreviewContentCard("", "html-doc");
    const contentCard = root.querySelector<HTMLElement>("[data-preview-content]");
    if (!contentCard) return;
    await renderHTMLPreview(contentCard, spec.raw || source.raw || "");
    if (token !== renderToken) return;
    await decorateRenderedDoc(contentCard, spec, source.path || "preview-html");
    if (token !== renderToken) return;
    contentCard.insertAdjacentHTML("afterbegin", metadata);
    return;
  }

  root.className = "preview-modal-body";
  root.innerHTML = renderPreviewContentCard(`${metadata}${renderCodePreview(spec.raw || source.raw || "", language)}`);
  decorateCodePreviewLines(root);
  scrollPreviewToLine(root, source.line);
}

async function decorateRenderedDoc(root: HTMLElement, spec: SpecDocument, fallbackKey: string): Promise<void> {
  await renderDiagramsIn(root, theme.value, spec.id || spec.path || fallbackKey);
  decorateInternalDocNavigation(root, spec, specs?.value || [], handleInternalLinkNavigation);
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
    <div class="modal-box preview-modal">
      <div class="preview-modal-header">
        <div class="min-w-0">
          <h2 id="previewDialogTitle" class="truncate text-sm font-semibold">{{ dialogTitle }}</h2>
          <p id="previewDialogPath" class="truncate text-xs text-c-text-secondary font-mono">{{ dialogPath }}</p>
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
