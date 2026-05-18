<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from "vue";
import Icon from "./Icon.vue";
import { renderNetworkGraph, type NetworkGraphData, type NetworkGraphLink, type NetworkGraphNode } from "../js/network_graph.js";

interface GraphNode {
  id: string;
  label?: string;
  type?: string;
  path?: string;
  specId?: string;
  category?: string;
  status?: string;
  [key: string]: any;
}

interface GraphEdge {
  from: string;
  to: string;
  type?: string;
  label?: string;
  [key: string]: any;
}

interface GraphData {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

interface Props {
  graph: GraphData | null;
  theme: "light" | "dark";
  active: boolean;
  query: string;
}

const props = defineProps<Props>();
const emit = defineEmits<{
  (e: "openSpecPreview", id: string): void;
  (e: "update:query", query: string): void;
}>();

const graphCanvas = ref<HTMLElement | null>(null);
const graphDetails = ref<HTMLElement | null>(null);
const graphSearch = ref("");
const graphStats = ref("Loading graph");
const selectedNodeId = ref("");
const graphRenderer = ref<ReturnType<typeof renderNetworkGraph> | null>(null);
const graphFullscreen = ref(false);
let renderedGraph: NetworkGraphData | null = null;

function nodeColor(node: GraphNode): string {
  if (node.type === "external") return "#94a3b8";
  switch (node.category) {
    case "modules":
      return "#2563eb";
    case "shared":
      return "#16a34a";
    case "decisions":
      return "#9333ea";
    case "planning":
      return "#f59e0b";
    case "compliance":
      return "#dc2626";
    default:
      return "#0f766e";
  }
}

function edgeColor(type: string): string {
  switch (type) {
    case "depends":
      return "#ef4444";
    case "implements":
      return "#6366f1";
    case "blocked-by":
      return "#f97316";
    case "verifies":
      return "#14b8a6";
    case "provides":
      return "#22c55e";
    case "consumes":
      return "#eab308";
    default:
      return "#64748b";
  }
}

function darkEdgeColor(type: string): string {
  switch (type) {
    case "depends":
      return "#f87171";
    case "implements":
      return "#818cf8";
    case "blocked-by":
      return "#fb923c";
    case "verifies":
      return "#2dd4bf";
    case "provides":
      return "#4ade80";
    case "consumes":
      return "#facc15";
    default:
      return "#94a3b8";
  }
}

function edgeColorForTheme(currentTheme: "light" | "dark"): (type: string) => string {
  return currentTheme === "dark" ? darkEdgeColor : edgeColor;
}

async function initGraph() {
  if (!graphCanvas.value || !props.graph) return;
  destroyGraph();
  const graph = normalizedGraphData(props.graph, graphSearch.value.trim().toLowerCase());
  renderedGraph = graph;
  graphStats.value = `${graph.nodes.length} nodes, ${graph.links.length} edges`;
  renderDetails(graph, selectedNodeId.value);
  graphCanvas.value.innerHTML = "";
  graphRenderer.value = renderNetworkGraph({
    container: graphCanvas.value,
    graph,
    selectedId: selectedNodeId.value,
    nodeColor,
    edgeColor: edgeColorForTheme(props.theme),
    labelColor: props.theme === "dark" ? "#f8fafc" : "#0f172a",
    unfocusedEdgeColor: props.theme === "dark" ? "#0f172a" : undefined,
    onSelectNode: selectGraphNode,
    onClearSelection: clearGraphSelection,
  });
}

function normalizedGraphData(raw: GraphData, query: string): NetworkGraphData {
  const visible = new Set<string>();
  const allNodes: NetworkGraphNode[] = (raw.nodes || []).map((node) => ({
    ...node,
    type: node.type || (node.specId ? "doc" : "external"),
    label: node.label || node.id,
  }));
  for (const node of allNodes) {
    const haystack = `${node.id} ${node.label || ""} ${node.path || ""} ${node.category || ""} ${node.status || ""}`.toLowerCase();
    if (!query || haystack.includes(query)) visible.add(node.id);
  }

  const links: NetworkGraphLink[] = (raw.edges || [])
    .map((edge) => ({ ...edge, source: edge.from, target: edge.to, type: edge.type || edge.label || "references" }))
    .filter((edge) => !query || visible.has(endpointID(edge.source)) || visible.has(endpointID(edge.target)));

  links.forEach((edge) => {
    visible.add(endpointID(edge.source));
    visible.add(endpointID(edge.target));
  });

  return {
    nodes: allNodes.filter((node) => visible.has(node.id)),
    links,
  };
}

function renderDetails(data: NetworkGraphData, nodeId: string) {
  if (!graphDetails.value) return;
  const node = data.nodes.find((n) => n.id === nodeId) || data.nodes[0];
  if (!node) {
    graphDetails.value.innerHTML = '<div class="p-4 text-sm text-base-content/60">No graph nodes.</div>';
    return;
  }
  const incoming = data.links.filter((edge) => endpointID(edge.target) === node.id);
  const outgoing = data.links.filter((edge) => endpointID(edge.source) === node.id);

  graphDetails.value.innerHTML = `
    <div class="grid gap-4 p-4">
      <div>
        <div class="text-xs uppercase tracking-wide text-base-content/50">${escapeHTML(node.type || "node")}</div>
        <h3 class="mt-1 text-lg font-semibold">${escapeHTML(node.label || node.id)}</h3>
        <p class="break-words text-sm text-base-content/60">${escapeHTML(node.path || node.id)}</p>
      </div>
      ${node.specId ? `<button class="btn btn-primary btn-sm" type="button" data-preview-spec="${escapeHTML(node.specId)}">Preview doc</button>` : ""}
      <div>
        <h4 class="mb-2 text-sm font-semibold">Outgoing refs (${outgoing.length})</h4>
        ${renderEdgeList(outgoing.slice(0, 12), "target")}
      </div>
      <div>
        <h4 class="mb-2 text-sm font-semibold">Incoming refs (${incoming.length})</h4>
        ${renderEdgeList(incoming.slice(0, 12), "source")}
      </div>
    </div>
  `;

  graphDetails.value.querySelector("[data-preview-spec]")?.addEventListener("click", (e) => {
    const el = e.currentTarget as HTMLElement;
    emit("openSpecPreview", el.dataset.previewSpec || "");
  });
  graphDetails.value.querySelectorAll<HTMLElement>("[data-select-node]").forEach((button) => {
    button.addEventListener("click", () => {
      const target = data.nodes.find((item) => item.id === button.dataset.selectNode);
      if (target) selectGraphNode(target);
    });
  });
}

function renderEdgeList(edges: NetworkGraphLink[], side: "source" | "target"): string {
  if (!edges.length) return '<div class="text-sm text-base-content/50">None</div>';
  return `
    <div class="grid gap-1">
      ${edges
        .map((edge) => {
          const related = endpointID(edge[side]);
          return `<button class="graph-ref-row" type="button" data-select-node="${escapeHTML(related)}">
          <span class="badge badge-ghost badge-sm">${escapeHTML(edge.type || "references")}</span>
          <span class="min-w-0 truncate">${escapeHTML(related)}</span>
        </button>`;
        })
        .join("")}
    </div>
  `;
}

function escapeHTML(str: string): string {
  return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#039;");
}

function endpointID(endpoint: string | NetworkGraphNode): string {
  return typeof endpoint === "string" ? endpoint : endpoint?.id || "";
}

function selectGraphNode(node: NetworkGraphNode) {
  selectedNodeId.value = node.id;
  graphRenderer.value?.setSelected(node.id);
  if (renderedGraph) renderDetails(renderedGraph, node.id);
}

function clearGraphSelection() {
  selectedNodeId.value = "";
  graphRenderer.value?.setSelected("");
  if (renderedGraph) renderDetails(renderedGraph, "");
}

function refitGraphSoon() {
  window.setTimeout(() => graphRenderer.value?.fit(), 60);
}

function toggleGraphFullscreen() {
  graphFullscreen.value = !graphFullscreen.value;
  refitGraphSoon();
}

function destroyGraph() {
  graphRenderer.value?.kill();
  graphRenderer.value = null;
  renderedGraph = null;
}

watch(
  () => props.graph,
  () => {
    if (!props.active) return;
    initGraph();
  },
  { immediate: true },
);

watch(
  () => props.active,
  (isActive) => {
    if (isActive) {
      initGraph();
    } else {
      destroyGraph();
    }
  },
);

watch(
  () => props.theme,
  () => {
    if (props.active) {
      initGraph();
    }
  },
);

watch(graphSearch, () => {
  emit("update:query", graphSearch.value);
  if (props.active) initGraph();
});

watch(
  () => props.query,
  (query) => {
    if (query === graphSearch.value) return;
    graphSearch.value = query || "";
  },
  { immediate: true },
);

onMounted(() => {
  if (props.graph && props.active) {
    initGraph();
  }
});

onUnmounted(() => {
  destroyGraph();
});
</script>

<template>
  <div class="graph-shell border-base-300 bg-base-100 border" :class="{ 'is-fullscreen': graphFullscreen }">
    <div class="graph-toolbar border-base-300 border-b">
      <div class="min-w-0">
        <h2 class="text-base font-semibold">Docs Graph</h2>
        <p id="graphStats" class="text-base-content/60 text-sm">{{ graphStats }}</p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <label class="input input-bordered input-sm flex items-center gap-2">
          <Icon name="search" class="text-base-content/50 h-4 w-4" />
          <input id="graphSearch" v-model="graphSearch" class="w-40 sm:w-56" placeholder="Find node" />
        </label>
        <button
          id="graphFit"
          class="btn btn-ghost btn-sm"
          type="button"
          aria-label="Fit graph"
          title="Fit graph"
          @click="graphRenderer?.fit()"
        >
          <Icon name="refresh-cw" class="h-4 w-4" />
        </button>
        <button
          id="graphFullscreen"
          class="btn btn-ghost btn-sm"
          type="button"
          :aria-label="graphFullscreen ? 'Exit full screen graph' : 'Full screen graph'"
          :title="graphFullscreen ? 'Exit full screen' : 'Full screen'"
          @click="toggleGraphFullscreen"
        >
          <Icon :name="graphFullscreen ? 'minimize' : 'maximize'" class="h-4 w-4" />
        </button>
      </div>
    </div>
    <div class="graph-layout">
      <div ref="graphCanvas" id="graphCanvas" class="graph-canvas" role="img" aria-label="Spec document graph"></div>
      <aside ref="graphDetails" id="graphDetails" class="graph-details"></aside>
    </div>
  </div>
</template>
