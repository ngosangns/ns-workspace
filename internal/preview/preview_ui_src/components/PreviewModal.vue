<script setup lang="ts">
import { ref, watch, computed, inject, nextTick, type Ref } from "vue";
import { decorateInternalDocNavigation, type SpecDocument, type InternalSpecTarget } from "../js/internal-links.js";

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
const previewDialogBody = ref<HTMLElement | null>(null);

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

const previewBody = computed(() => {
  if (!props.source) return "";
  if (props.showRaw) {
    return escapeCodePreview(props.source.raw, props.source.language);
  }
  return props.source.raw;
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

function escapeCodePreview(raw: string, language: string): string {
  return `<pre class="bg-base-300 rounded p-4 overflow-auto"><code>${escapeHTML(raw)}</code></pre>`;
}

watch(
  () => [props.source, props.showRaw] as const,
  async () => {
    await nextTick();
    if (previewDialogBody.value && props.source && !props.showRaw && props.source.spec) {
      decorateInternalDocNavigation(previewDialogBody.value, props.source.spec, specs?.value || [], handleInternalLinkNavigation);
    }
  },
  { flush: "post" },
);
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
            <i data-lucide="file-code" class="h-4 w-4"></i>
          </button>
          <button
            class="btn btn-ghost btn-sm"
            type="button"
            data-close-preview
            aria-label="Close preview"
            title="Close preview"
            @click="emit('close')"
          >
            <i data-lucide="x" class="h-4 w-4"></i>
          </button>
        </div>
      </div>
      <div id="previewDialogBody" ref="previewDialogBody" class="preview-modal-body" v-html="previewBody"></div>
    </div>
    <button class="modal-backdrop" type="button" @click="emit('close')">close</button>
  </div>
</template>
