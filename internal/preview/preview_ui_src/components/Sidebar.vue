<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from "vue";
import TreeNode from "./TreeNode.vue";
import Icon from "./Icon.vue";

interface ProjectSummary {
  name: string;
  generatedTitle?: string;
  projectRoot?: string;
  docsRoot?: string;
  totalSpecs: number;
}

interface SpecDocument {
  id: string;
  title: string;
  path: string;
  status?: string;
  compliance?: string;
}

const props = defineProps<{
  project: ProjectSummary | null;
  specs: SpecDocument[];
  selectedId: string;
  selectedFolderPath: string;
}>();

const emit = defineEmits<{
  (e: "selectSpec", id: string): void;
}>();

const search = ref("");
const expandedPaths = ref(new Set<string>());
const sidebarWidth = ref(352);
const isResizing = ref(false);
const sidebarWidthStorageKey = "spec-preview-sidebar-width";
const minSidebarWidth = 256;
const maxSidebarWidth = 560;

const filteredSpecs = computed(() => {
  const query = search.value.toLowerCase().trim();
  if (!query) return props.specs;
  return props.specs.filter((spec) => {
    const haystack = `${spec.title} ${spec.path} ${spec.status} ${spec.compliance}`.toLowerCase();
    return haystack.includes(query);
  });
});

const tree = computed(() => buildSpecTree(filteredSpecs.value));

function buildSpecTree(specs: SpecDocument[]) {
  const root: any = { name: "", path: "", type: "folder", children: new Map() };
  specs.forEach((spec) => {
    const parts = spec.path.split("/");
    let cursor = root;
    parts.forEach((part, index) => {
      const isFile = index === parts.length - 1;
      const path = parts.slice(0, index + 1).join("/");
      if (!cursor.children.has(part)) {
        cursor.children.set(
          part,
          isFile ? { name: part, path, type: "file", spec } : { name: part, path, type: "folder", children: new Map() },
        );
      }
      cursor = cursor.children.get(part);
      if (isFile) {
        cursor.spec = spec;
      }
    });
  });
  sortTree(root);
  return root;
}

function sortTree(node: any) {
  if (!node.children) return;
  node.children = new Map(
    [...node.children.entries()].sort(([, a]: [string, any], [, b]: [string, any]) => {
      if (a.type !== b.type) return a.type === "folder" ? -1 : 1;
      return a.name.localeCompare(b.name);
    }),
  );
  node.children.forEach(sortTree);
}

function toggleFolder(path: string) {
  if (expandedPaths.value.has(path)) {
    expandedPaths.value.delete(path);
  } else {
    expandedPaths.value.add(path);
  }
}

function handleSelectSpec(idOrPath: string) {
  emit("selectSpec", idOrPath);
}

function autoExpandForSelection() {
  const activeId = props.selectedId || props.selectedFolderPath;
  if (!activeId) return;
  const parts = activeId.split("/");
  for (let index = 1; index < parts.length; index++) {
    expandedPaths.value.add(parts.slice(0, index).join("/"));
  }
}

function clampSidebarWidth(width: number): number {
  return Math.min(maxSidebarWidth, Math.max(minSidebarWidth, Math.round(width)));
}

function applySidebarWidth(width: number) {
  sidebarWidth.value = clampSidebarWidth(width);
  // Keep the fixed sidebar and the main content offset in sync across components.
  document.documentElement.style.setProperty("--preview-sidebar-width", `${sidebarWidth.value}px`);
}

function persistSidebarWidth() {
  localStorage.setItem(sidebarWidthStorageKey, String(sidebarWidth.value));
}

function restoreSidebarWidth() {
  const stored = Number(localStorage.getItem(sidebarWidthStorageKey) || "");
  if (Number.isFinite(stored) && stored > 0) {
    applySidebarWidth(stored);
    return;
  }
  applySidebarWidth(sidebarWidth.value);
}

function handleResizePointerMove(event: PointerEvent) {
  if (!isResizing.value) return;
  applySidebarWidth(event.clientX);
}

function stopSidebarResize() {
  if (!isResizing.value) return;
  isResizing.value = false;
  document.body.classList.remove("is-resizing-sidebar");
  persistSidebarWidth();
}

function startSidebarResize(event: PointerEvent) {
  if (event.button !== 0) return;
  isResizing.value = true;
  document.body.classList.add("is-resizing-sidebar");
  applySidebarWidth(event.clientX);
  (event.currentTarget as HTMLElement).setPointerCapture(event.pointerId);
}

function nudgeSidebarWidth(delta: number) {
  applySidebarWidth(sidebarWidth.value + delta);
  persistSidebarWidth();
}

function handleResizeKeydown(event: KeyboardEvent) {
  if (event.key === "ArrowLeft") {
    event.preventDefault();
    nudgeSidebarWidth(event.shiftKey ? -48 : -16);
  } else if (event.key === "ArrowRight") {
    event.preventDefault();
    nudgeSidebarWidth(event.shiftKey ? 48 : 16);
  } else if (event.key === "Home") {
    event.preventDefault();
    applySidebarWidth(minSidebarWidth);
    persistSidebarWidth();
  } else if (event.key === "End") {
    event.preventDefault();
    applySidebarWidth(maxSidebarWidth);
    persistSidebarWidth();
  }
}

onMounted(() => {
  restoreSidebarWidth();
  window.addEventListener("pointermove", handleResizePointerMove);
  window.addEventListener("pointerup", stopSidebarResize);
  window.addEventListener("pointercancel", stopSidebarResize);
});

onUnmounted(() => {
  window.removeEventListener("pointermove", handleResizePointerMove);
  window.removeEventListener("pointerup", stopSidebarResize);
  window.removeEventListener("pointercancel", stopSidebarResize);
  document.body.classList.remove("is-resizing-sidebar");
});

watch(
  () => props.selectedId,
  () => autoExpandForSelection(),
  { immediate: true },
);
</script>

<template>
  <aside
    class="preview-sidebar border-base-300 bg-base-100 max-h-[46vh] overflow-auto border-b p-4 lg:fixed lg:left-0 lg:top-0 lg:h-screen lg:max-h-none lg:border-b-0 lg:border-r"
  >
    <div class="mb-4 flex items-center gap-3">
      <div class="grid h-10 w-10 place-items-center rounded-lg bg-neutral text-neutral-content">
        <Icon name="book-open-text" class="h-5 w-5" />
      </div>
      <div class="min-w-0">
        <div class="font-bold">Docs Preview</div>
        <div id="projectName" class="text-base-content/60 truncate text-sm">
          {{ project?.name || "Loading" }}
        </div>
        <div id="projectPath" class="text-base-content/50 truncate text-xs">
          {{ project?.projectRoot || "" }}
        </div>
      </div>
    </div>

    <label class="input input-bordered mb-3 flex h-10 items-center gap-2">
      <Icon name="search" class="text-base-content/50 h-4 w-4" />
      <input id="search" v-model="search" class="grow" placeholder="Doc name, path, status" />
    </label>

    <nav id="specList" class="space-y-1">
      <TreeNode
        v-for="[name, node] in Array.from(tree.children as Map<string, any>)"
        :key="name"
        :node="node"
        :selected-id="selectedId"
        :selected-folder-path="selectedFolderPath"
        :expanded-paths="expandedPaths"
        :depth="0"
        @select-spec="handleSelectSpec"
        @toggle-folder="toggleFolder"
      />
    </nav>

    <div
      class="sidebar-resize-handle hidden lg:block"
      role="separator"
      aria-orientation="vertical"
      aria-label="Resize sidebar"
      tabindex="0"
      :aria-valuemin="minSidebarWidth"
      :aria-valuemax="maxSidebarWidth"
      :aria-valuenow="sidebarWidth"
      @pointerdown="startSidebarResize"
      @keydown="handleResizeKeydown"
    ></div>
  </aside>
</template>
