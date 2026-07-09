<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, computed } from "vue";
import { EditorView, keymap, placeholder as placeholderExt } from "@codemirror/view";
import { EditorState, Compartment, type Extension } from "@codemirror/state";
import { indentWithTab } from "@codemirror/commands";
import { json } from "@codemirror/lang-json";
import { markdown } from "@codemirror/lang-markdown";
import { oneDark } from "@codemirror/theme-one-dark";
import { linter, lintGutter } from "@codemirror/lint";

interface Props {
  modelValue: string;
  lang?: "json" | "markdown" | "text";
  readonly?: boolean;
  dark?: boolean;
  placeholder?: string;
}

const props = withDefaults(defineProps<Props>(), {
  lang: "text",
  readonly: false,
  dark: true,
  placeholder: "",
});

const emit = defineEmits<{
  (e: "update:modelValue", value: string): void;
}>();

const host = ref<HTMLElement | null>(null);
let view: EditorView | null = null;

const readOnlyCompartment = new Compartment();
const editableCompartment = new Compartment();
const languageCompartment = new Compartment();
const themeCompartment = new Compartment();

const jsonLinter = linter((view) => {
  const doc = view.state.doc.toString();
  try {
    JSON.parse(doc);
    return [];
  } catch (err) {
    const message = err instanceof Error ? err.message : "Invalid JSON";
    const match = message.match(/position (\d+)/);
    const pos = match ? Number.parseInt(match[1], 10) : 0;
    const line = view.state.doc.lineAt(pos);
    return [
      {
        from: line.from,
        to: line.to,
        severity: "error",
        message,
      },
    ];
  }
});

function languageExtension(): Extension[] {
  switch (props.lang) {
    case "json":
      return [json(), jsonLinter, lintGutter()];
    case "markdown":
      return [markdown()];
    default:
      return [];
  }
}

function themeExtension(): Extension[] {
  return props.dark ? [oneDark] : [];
}

function createStaticExtensions(): Extension[] {
  const extensions: Extension[] = [
    readOnlyCompartment.of(EditorState.readOnly.of(props.readonly)),
    editableCompartment.of(EditorView.editable.of(!props.readonly)),
    languageCompartment.of(languageExtension()),
    themeCompartment.of(themeExtension()),
    keymap.of([indentWithTab]),
    EditorView.updateListener.of((update) => {
      if (update.docChanged) {
        const value = update.state.doc.toString();
        if (value !== props.modelValue) {
          emit("update:modelValue", value);
        }
      }
    }),
  ];

  if (props.placeholder) {
    extensions.push(placeholderExt(props.placeholder));
  }

  return extensions;
}

function initEditor() {
  if (!host.value) return;

  view = new EditorView({
    doc: props.modelValue,
    extensions: createStaticExtensions(),
    parent: host.value,
  });
}

watch(
  () => props.modelValue,
  (value) => {
    if (!view) return;
    const current = view.state.doc.toString();
    if (value === current) return;
    view.dispatch({
      changes: { from: 0, to: current.length, insert: value },
    });
  },
);

watch(
  () => props.readonly,
  (readonly) => {
    view?.dispatch({
      effects: [
        readOnlyCompartment.reconfigure(EditorState.readOnly.of(readonly)),
        editableCompartment.reconfigure(EditorView.editable.of(!readonly)),
      ],
    });
  },
);

watch(
  () => props.dark,
  (dark) => {
    view?.dispatch({
      effects: themeCompartment.reconfigure(dark ? [oneDark] : []),
    });
  },
);

watch(
  () => props.lang,
  () => {
    view?.dispatch({
      effects: languageCompartment.reconfigure(languageExtension()),
    });
  },
);

onMounted(initEditor);
onUnmounted(() => {
  view?.destroy();
  view = null;
});

const wrapperClass = computed(() => ({
  "code-editor": true,
  "code-editor--readonly": props.readonly,
  "code-editor--dark": props.dark,
}));
</script>

<template>
  <div ref="host" :class="wrapperClass" />
</template>

<style scoped>
.code-editor {
  font-family: var(--font-mono, monospace);
  font-size: 13px;
  line-height: 1.6;
  overflow: hidden;
}

.code-editor :deep(.cm-editor) {
  min-height: 500px;
  background: var(--color-app);
}

.code-editor :deep(.cm-editor.cm-focused) {
  outline: none;
}

.code-editor :deep(.cm-scroller) {
  font-family: var(--font-mono, monospace);
  font-variant-numeric: tabular-nums;
}

.code-editor :deep(.cm-gutters) {
  border-right: 1px solid var(--color-border);
}

.code-editor--readonly :deep(.cm-cursor) {
  display: none;
}
</style>
