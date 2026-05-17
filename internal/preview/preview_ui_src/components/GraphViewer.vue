<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from "vue";

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
}

const props = defineProps<Props>();
const emit = defineEmits<{
  (e: "openSpecPreview", id: string): void;
}>();

const graphCanvas = ref<HTMLElement | null>(null);
const graphDetails = ref<HTMLElement | null>(null);
const graphSearch = ref("");
const graphStats = ref("Loading graph");
const selectedNodeId = ref("");
const graphRenderer = ref<any>(null);
const graphSearchFiltered = ref<GraphData | null>(null);

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

async function initGraph() {
  if (!graphCanvas.value || !props.graph) return;
  const Graph = (await import("graphology")).default;
  const forceAtlas2 = (await import("graphology-layout-forceatlas2")).default;
  const Sigma = (await import("sigma")).default;

  const data = graphSearchFiltered.value || props.graph;
  const graph = new Graph({ multi: true, type: "directed" });

  const visible = new Set<string>();
  const allNodes = (data.nodes || []).map((node) => ({
    ...node,
    type: node.type || (node.specId ? "doc" : "external"),
    label: node.label || node.id,
  }));

  const query = graphSearch.value.trim().toLowerCase();
  for (const node of allNodes) {
    if (!query) {
      visible.add(node.id);
    } else {
      const haystack = `${node.id} ${node.label} ${node.path || ""} ${node.category || ""} ${node.status || ""}`.toLowerCase();
      if (haystack.includes(query)) visible.add(node.id);
    }
  }

  allNodes.forEach((node) => {
    if (visible.has(node.id)) {
      graph.addNode(node.id, {
        color: nodeColor(node),
        label: node.label || node.id,
        x: 0,
        y: 0,
        size: 8,
      });
    }
  });

  const links = (data.edges || [])
    .map((edge) => ({ ...edge, source: edge.from, target: edge.to, type: edge.type || edge.label || "references" }))
    .filter((edge) => visible.has(edge.source) && visible.has(edge.target));

  let serial = 0;
  for (const link of links) {
    if (!graph.hasNode(link.source) || !graph.hasNode(link.target)) continue;
    const key = `edge:${link.source}:${link.target}:${link.type}:${serial++}`;
    graph.addDirectedEdgeWithKey(key, link.source, link.target, {
      color: edgeColor(link.type || "references"),
      label: link.type || "references",
      size: 1.5,
    });
  }

  if (graph.order > 1) {
    const iterations = graph.order > 500 ? 80 : graph.order > 180 ? 110 : 160;
    const settings = forceAtlas2.inferSettings(graph);
    forceAtlas2.assign(graph, {
      iterations,
      settings: {
        ...settings,
        barnesHutOptimize: graph.order > 120,
        edgeWeightInfluence: 0.35,
        gravity: graph.order > 160 ? 1.2 : 0.8,
        scalingRatio: graph.order > 160 ? 12 : 8,
        slowDown: 8,
      },
    });
  }

  graphStats.value = `${graph.order} nodes, ${graph.size} edges`;

  const renderer = new Sigma(graph, graphCanvas.value, {
    allowInvalidContainer: true,
    autoCenter: true,
    autoRescale: true,
    defaultEdgeType: "arrow",
    defaultNodeColor: "#94a3b8",
    labelColor: { attribute: "labelColor", color: props.theme === "dark" ? "#f8fafc" : "#0f172a" },
    labelSize: 12,
    maxCameraRatio: 10,
    minCameraRatio: 0.05,
  });

  let currentSelected = selectedNodeId.value && graph.hasNode(selectedNodeId.value) ? selectedNodeId.value : "";

  renderer.on("clickNode", ({ node }: { node: string }) => {
    currentSelected = node;
    selectedNodeId.value = node;
    renderDetails(data, node);
    renderer.refresh();
  });

  renderer.on("clickStage", () => {
    if (!currentSelected) return;
    currentSelected = "";
    selectedNodeId.value = "";
    renderDetails(data, "");
    renderer.refresh();
  });

  graphRenderer.value = {
    fit: () => {
      renderer.getCamera().animatedReset({ duration: 260 });
    },
    kill: () => renderer.kill(),
    setSelected: (id: string) => {
      currentSelected = graph.hasNode(id) ? id : "";
      renderer.refresh();
    },
  };

  if (currentSelected) {
    renderDetails(data, currentSelected);
  } else if (graph.order > 0) {
    const firstNode = graph.nodes()[0];
    if (firstNode) {
      renderDetails(data, firstNode);
    }
  }
}

function renderDetails(data: GraphData, nodeId: string) {
  if (!graphDetails.value) return;
  const node = data.nodes.find((n) => n.id === nodeId);
  if (!node) {
    graphDetails.value.innerHTML = '<div class="p-4 text-sm text-base-content/60">No graph nodes.</div>';
    return;
  }
  const incoming = data.edges.filter((e) => e.to === nodeId);
  const outgoing = data.edges.filter((e) => e.from === nodeId);

  graphDetails.value.innerHTML = `
    <div class="grid gap-4 p-4">
      <div>
        <div class="text-xs uppercase tracking-wide text-base-content/50">${escapeHTML(node.type || "node")}</div>
        <h3 class="mt-1 text-lg font-semibold">${escapeHTML(node.label || node.id)}</h3>
        <p class="break-words text-sm text-base-content/60">${escapeHTML(node.path || node.id)}</p>
      </div>
      ${node.specId ? `<button class="btn btn-primary btn-sm" type="button" data-preview-spec="${escapeHTML(node.specId)}"><i data-lucide="file-text" class="h-4 w-4"></i>Preview doc</button>` : ""}
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
}

function renderEdgeList(edges: GraphEdge[], side: "source" | "target"): string {
  if (!edges.length) return '<div class="text-sm text-base-content/50">None</div>';
  return `
    <div class="grid gap-1">
      ${edges
        .map((edge) => {
          const related = edge[side] === "string" ? edge[side] : (edge[side] as any)?.id || "";
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

watch(
  () => props.graph,
  () => {
    if (!props.active) return;
    graphRenderer.value?.kill();
    graphRenderer.value = null;
    initGraph();
  },
  { immediate: true },
);

watch(
  () => props.active,
  (isActive) => {
    if (isActive) {
      graphRenderer.value?.kill();
      graphRenderer.value = null;
      initGraph();
    } else {
      graphRenderer.value?.kill();
      graphRenderer.value = null;
    }
  },
);

watch(
  () => props.theme,
  () => {
    if (graphRenderer.value && props.active) {
      initGraph();
    }
  },
);

onMounted(() => {
  if (props.graph && props.active) {
    initGraph();
  }
});

onUnmounted(() => {
  graphRenderer.value?.kill();
});
</script>

<template>
  <div class="graph-shell border-base-300 bg-base-100 border">
    <div class="graph-toolbar border-base-300 border-b">
      <div class="min-w-0">
        <h2 class="text-base font-semibold">Docs Graph</h2>
        <p id="graphStats" class="text-base-content/60 text-sm">{{ graphStats }}</p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <label class="input input-bordered input-sm flex items-center gap-2">
          <i data-lucide="search" class="text-base-content/50 h-4 w-4"></i>
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
          <i data-lucide="refresh-cw" class="h-4 w-4"></i>
        </button>
      </div>
    </div>
    <div class="graph-layout">
      <div ref="graphCanvas" id="graphCanvas" class="graph-canvas" role="img" aria-label="Spec document graph"></div>
      <aside ref="graphDetails" id="graphDetails" class="graph-details"></aside>
    </div>
  </div>
</template>
