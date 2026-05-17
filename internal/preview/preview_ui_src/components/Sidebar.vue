<script setup lang="ts">
import { ref, computed, watch } from "vue";
import TreeNode from "./TreeNode.vue";

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

watch(
  () => props.selectedId,
  () => autoExpandForSelection(),
  { immediate: true },
);
</script>

<template>
  <aside
    class="border-base-300 bg-base-100 max-h-[46vh] overflow-auto border-b p-4 lg:fixed lg:left-0 lg:top-0 lg:h-screen lg:w-[22rem] lg:max-h-none lg:border-b-0 lg:border-r"
  >
    <div class="mb-4 flex items-center gap-3">
      <div class="grid h-10 w-10 place-items-center rounded-lg bg-neutral text-neutral-content">
        <i data-lucide="book-open-text" class="h-5 w-5"></i>
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
      <i data-lucide="search" class="text-base-content/50 h-4 w-4"></i>
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
  </aside>
</template>
