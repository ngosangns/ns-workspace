<script setup lang="ts">
import { computed } from "vue";
import Icon from "./Icon.vue";

interface SpecDocument {
  id: string;
  title: string;
  path: string;
  status?: string;
  compliance?: string;
}

interface TreeNodeData {
  name: string;
  path: string;
  type: "folder" | "file";
  children?: Map<string, TreeNodeData>;
  spec?: SpecDocument;
}

const props = defineProps<{
  node: TreeNodeData;
  selectedId: string;
  selectedFolderPath: string;
  expandedPaths: Set<string>;
  depth?: number;
}>();

const emit = defineEmits<{
  (e: "selectSpec", id: string): void;
  (e: "toggleFolder", path: string): void;
}>();

const paddingLeft = computed(() => `${(props.depth || 0) * 16 + 8}px`);

function displaySpecName(spec: SpecDocument): string {
  const base = spec.path.split("/").pop() || spec.title;
  if (base === "_overview.md") return spec.title;
  return spec.title || base;
}

function handleFolderClick(path: string) {
  emit("selectSpec", path);
  emit("toggleFolder", path);
}

function handleToggleFolder(path: string, event: Event) {
  event.stopPropagation();
  emit("toggleFolder", path);
}

function isExpanded(path: string): boolean {
  return props.expandedPaths.has(path);
}
</script>

<template>
  <div>
    <button
      v-if="node.type === 'folder'"
      class="tree-row btn btn-ghost btn-sm min-h-8 w-full justify-start gap-1 overflow-hidden px-2 text-left font-medium"
      :class="{ 'btn-active': selectedFolderPath === node.path }"
      :style="{ paddingLeft: paddingLeft }"
      :title="node.path"
      @click="handleFolderClick(node.path)"
    >
      <Icon
        name="chevron-right"
        class="tree-chevron h-4 w-4 shrink-0 transition-transform"
        :class="{ 'rotate-90': isExpanded(node.path) }"
        @click="handleToggleFolder(node.path, $event)"
      />
      <Icon name="folder" class="h-4 w-4 shrink-0 text-base-content/60" />
      <span class="min-w-0 flex-1 truncate">{{ node.name }}</span>
    </button>
    <button
      v-else-if="node.spec"
      class="tree-row btn btn-ghost btn-sm min-h-8 w-full justify-start gap-1 overflow-hidden px-2 text-left font-normal"
      :class="{ 'btn-active': selectedId === node.spec.id }"
      :style="{ paddingLeft: `${(depth || 0) * 16 + 24}px` }"
      :title="node.spec.path"
      @click="emit('selectSpec', node.spec.id)"
    >
      <Icon name="file-text" class="h-4 w-4 shrink-0 text-base-content/55" />
      <span class="min-w-0 flex-1 truncate">{{ displaySpecName(node.spec) }}</span>
    </button>

    <!-- Render children recursively when folder is expanded -->
    <template v-if="node.type === 'folder' && isExpanded(node.path) && node.children">
      <TreeNode
        v-for="[childName, childNode] in Array.from(node.children as Map<string, TreeNodeData>)"
        :key="childName"
        :node="childNode"
        :selected-id="selectedId"
        :selected-folder-path="selectedFolderPath"
        :expanded-paths="expandedPaths"
        :depth="(depth || 0) + 1"
        @select-spec="emit('selectSpec', $event)"
        @toggle-folder="emit('toggleFolder', $event)"
      />
    </template>
  </div>
</template>
