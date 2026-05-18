<script setup lang="ts">
import { inject, nextTick, onUnmounted, ref, watch, type Ref } from "vue";
import Icon from "./Icon.vue";
import { renderNetworkGraph, type NetworkGraphData, type NetworkGraphLink, type NetworkGraphNode } from "../js/network_graph.js";

interface SearchResult {
  id?: string;
  nodeId?: string;
  title?: string;
  path?: string;
  line?: number;
  specId?: string;
  description?: string;
  excerpt?: string;
  kind?: string;
  score?: number;
  community?: string;
  relation?: string;
  confidence?: string;
  anchor?: boolean;
  anchorId?: string;
  depth?: number;
  matchedBy?: string[];
  neighbors?: SearchNeighbor[];
}

interface SearchNeighbor {
  id?: string;
  label?: string;
  path?: string;
  line?: number;
  relation?: string;
  confidence?: string;
}

interface SearchResponse {
  query?: string;
  mode?: string;
  keywordOperator?: string;
  warnings?: string[];
  stats?: Record<string, number>;
  panels?: Record<string, SearchResult[]>;
}

interface Props {
  query: string;
  keywordOperator: string;
}

const props = defineProps<Props>();
const emit = defineEmits<{
  (e: "openSpecPreview", id: string): void;
  (e: "openFilePreview", path: string, line: number): void;
  (e: "update:query", query: string): void;
  (e: "update:keywordOperator", keywordOperator: string): void;
}>();

const searchQuery = ref("");
const keywordOperator = ref("sum");
const searchLoading = ref(false);
const searchData = ref<SearchResponse | null>(null);
const searchTimer = ref<ReturnType<typeof setTimeout> | null>(null);
const searchController = ref<AbortController | null>(null);
const docsGraphCanvas = ref<HTMLElement | null>(null);
const docsGraphDetails = ref<HTMLElement | null>(null);
const codeGraphCanvas = ref<HTMLElement | null>(null);
const codeGraphDetails = ref<HTMLElement | null>(null);
const theme = inject<Ref<"light" | "dark">>("theme", ref("light"));
const searchGraphRenderers = new Map<string, ReturnType<typeof renderNetworkGraph>>();
const searchGraphSelections = new Map<string, string>();
const renderedSearchGraphs = new Map<string, NetworkGraphData>();
const fullscreenSearchGraph = ref("");

async function fetchJSON(path: string) {
  const res = await fetch(path);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

async function runSearch() {
  const query = searchQuery.value.trim();
  if (!query) {
    searchData.value = null;
    searchLoading.value = false;
    stopAllSearchGraphs();
    return;
  }
  if (searchController.value) {
    searchController.value.abort();
  }
  searchLoading.value = true;
  searchController.value = new AbortController();
  const params = new URLSearchParams({ q: query, limit: "8" });
  if (keywordOperator.value !== "sum") {
    params.set("keywordOp", keywordOperator.value);
  }
  try {
    const data = await fetchJSON(`/api/search?${params.toString()}`);
    searchData.value = data;
    searchLoading.value = false;
    await renderAllSearchGraphs();
  } catch (error) {
    if (error instanceof Error && error.name !== "AbortError") {
      searchLoading.value = false;
      console.error(error);
    }
  }
}

function scheduleSearch() {
  window.clearTimeout(searchTimer.value || undefined);
  searchTimer.value = setTimeout(() => runSearch(), 180);
}

watch(searchQuery, () => {
  emit("update:query", searchQuery.value);
  scheduleSearch();
});

watch(keywordOperator, () => {
  emit("update:keywordOperator", keywordOperator.value);
  scheduleSearch();
});

watch(
  () => props.query,
  (query) => {
    if (query === searchQuery.value) return;
    searchQuery.value = query || "";
  },
  { immediate: true },
);

watch(
  () => props.keywordOperator,
  (operator) => {
    const next = operator === "difference" ? "difference" : "sum";
    if (next === keywordOperator.value) return;
    keywordOperator.value = next;
  },
  { immediate: true },
);

watch(theme, () => {
  void renderAllSearchGraphs();
});

function renderSearchSummary(): string {
  if (searchLoading.value) {
    return `
      <div class="flex flex-wrap items-center gap-2" aria-live="polite">
        <span class="loading loading-spinner loading-sm text-primary"></span>
        <span class="text-sm text-base-content/70">Searching docs, code, and graphs...</span>
      </div>
    `;
  }
  if (!searchData.value) {
    return '<span class="text-sm text-base-content/60">Search across docs, code, and graph context.</span>';
  }
  const stats = searchData.value.stats || {};
  const total = Object.values(stats).reduce((sum, v) => sum + Number(v || 0), 0);
  const warnings = searchData.value.warnings || [];
  return `
    <div class="flex flex-wrap items-center gap-2">
      <span class="badge badge-primary badge-sm">${escapeHTML(searchData.value.mode || "hybrid")}</span>
      <span class="badge badge-ghost badge-sm">${keywordOperator.value === "difference" ? "keyword difference" : "keyword sum"}</span>
      <span class="badge badge-ghost badge-sm">${total} results</span>
      ${warnings
        .slice(0, 2)
        .map((w) => `<span class="badge badge-warning badge-sm max-w-full truncate">${escapeHTML(w)}</span>`)
        .join("")}
    </div>
  `;
}

function panelResults(panelName: string): SearchResult[] {
  return searchData.value?.panels?.[panelName] || [];
}

async function renderAllSearchGraphs() {
  await nextTick();
  renderSearchGraphPanel("docsGraph", panelResults("docsGraph"), docsGraphCanvas.value, docsGraphDetails.value);
  renderSearchGraphPanel("codeGraph", panelResults("codeGraph"), codeGraphCanvas.value, codeGraphDetails.value);
}

function renderSearchGraphPanel(name: string, results: SearchResult[], canvas: HTMLElement | null, details: HTMLElement | null) {
  stopSearchGraph(name);
  if (!results.length || !canvas || !details) {
    renderedSearchGraphs.delete(name);
    return;
  }

  const graph = searchResultsToGraph(results, name);
  renderedSearchGraphs.set(name, graph);
  renderSearchGraphDetails(name, graph, details);
  canvas.innerHTML = "";
  const selected = searchGraphSelections.get(name) || "";
  const renderer = renderNetworkGraph({
    container: canvas,
    graph,
    selectedId: selected,
    nodeColor: searchNodeColor,
    edgeColor: searchEdgeColorForTheme(theme.value),
    labelColor: theme.value === "dark" ? "#f8fafc" : "#0f172a",
    unfocusedEdgeColor: theme.value === "dark" ? "#0f172a" : undefined,
    onSelectNode: (item) => selectSearchGraphNode(name, item.id),
    onClearSelection: () => clearSearchGraphSelection(name),
  });
  searchGraphRenderers.set(name, renderer);
  if (fullscreenSearchGraph.value === name) {
    refitSearchGraphSoon(name);
  }
}

function searchResultsToGraph(results: SearchResult[], panelName: string): NetworkGraphData {
  const nodes = new Map<string, NetworkGraphNode>();
  const links: NetworkGraphLink[] = [];
  const ensureNode = (node: NetworkGraphNode) => {
    const existing: Partial<NetworkGraphNode> = nodes.get(node.id) || {};
    nodes.set(node.id, { ...existing, ...node, label: node.label || existing.label || node.id });
  };
  const addLink = (source: string, target: string, type?: string, confidence?: string) => {
    if (!source || !target || source === target) return;
    links.push({ source, target, type: type || "references", confidence: confidence || "" });
  };

  results.forEach((result, index) => {
    const resultID = result.nodeId || result.id || `${panelName}:${index}`;
    const fileName = result.path ? result.path.split("/").pop() || "" : "";
    ensureNode({
      id: resultID,
      label: panelName === "codeGraph" ? codeGraphNodeLabel(result, fileName) : result.title || result.nodeId || result.path || result.id,
      type: panelName === "codeGraph" ? "code" : "doc",
      path: result.path || "",
      line: result.line || 0,
      previewPath: result.path || "",
      previewLine: result.line || 0,
      specId: result.specId || "",
      community: result.community || "",
      score: result.score || 0,
      result,
    });
    if (result.path) {
      const fileID = `file:${result.path}`;
      ensureNode({
        id: fileID,
        label: fileName,
        type: result.specId ? "doc-file" : "file",
        path: result.path,
        line: result.line || 0,
        previewPath: result.path,
        previewLine: result.line || 0,
        specId: result.specId || "",
      });
      addLink(fileID, resultID, result.specId ? "documents" : "defines", result.confidence);
    }
    (result.neighbors || []).forEach((neighbor) => {
      const neighborID = neighbor.id || neighbor.label || "";
      const neighborPath = neighbor.path || (panelName === "codeGraph" ? result.path || "" : "");
      const neighborLine = Number(neighbor.line || (panelName === "codeGraph" ? result.line || 0 : 0));
      ensureNode({
        id: neighborID,
        label: neighbor.label || neighbor.id || neighborID,
        type: "flow",
        path: neighborPath,
        previewPath: neighborPath,
        previewLine: neighborLine,
        line: neighborLine,
        confidence: neighbor.confidence || "",
      });
      addLink(resultID, neighborID, neighbor.relation, neighbor.confidence);
    });
  });

  return { nodes: [...nodes.values()], links: dedupeGraphLinks(links) };
}

function dedupeGraphLinks(links: NetworkGraphLink[]): NetworkGraphLink[] {
  const seen = new Set<string>();
  return links.filter((link) => {
    const key = `${endpointID(link.source)}->${endpointID(link.target)}:${link.type || "references"}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

function selectSearchGraphNode(name: string, nodeId: string) {
  const graph = renderedSearchGraphs.get(name);
  const details = searchGraphDetailsElement(name);
  const node = graph?.nodes.find((item) => item.id === nodeId);
  if (!graph || !details || !node) return;
  searchGraphSelections.set(name, node.id);
  searchGraphRenderers.get(name)?.setSelected(node.id);
  renderSearchGraphDetails(name, graph, details);
}

function clearSearchGraphSelection(name: string) {
  const graph = renderedSearchGraphs.get(name);
  const details = searchGraphDetailsElement(name);
  if (!graph || !details) return;
  searchGraphSelections.delete(name);
  searchGraphRenderers.get(name)?.setSelected("");
  renderSearchGraphDetails(name, graph, details);
}

function renderSearchGraphDetails(name: string, graph: NetworkGraphData, details: HTMLElement) {
  const selected = searchGraphSelections.get(name) || "";
  const node = graph.nodes.find((item) => item.id === selected) || graph.nodes[0];
  if (!node) {
    details.innerHTML = '<div class="p-4 text-sm text-base-content/60">No graph results.</div>';
    return;
  }

  const incoming = graph.links.filter((edge) => endpointID(edge.target) === node.id);
  const outgoing = graph.links.filter((edge) => endpointID(edge.source) === node.id);
  details.innerHTML = `
    <div class="grid gap-3 p-3">
      <div>
        <div class="text-xs uppercase tracking-wide text-base-content/50">${escapeHTML(node.type || "node")}</div>
        <h3 class="mt-1 text-sm font-semibold">${escapeHTML(node.label || node.id)}</h3>
        <p class="break-words text-xs text-base-content/60">${escapeHTML(node.path || node.id)}</p>
      </div>
      <div class="flex flex-wrap gap-2">
        ${node.specId ? `<button class="btn btn-primary btn-xs" type="button" data-preview-spec="${escapeHTML(node.specId)}">Preview doc</button>` : ""}
      </div>
      <div>
        <h4 class="mb-1 text-xs font-semibold">Outgoing flows (${outgoing.length})</h4>
        ${renderSearchGraphEdgeList(outgoing, "target")}
      </div>
      <div>
        <h4 class="mb-1 text-xs font-semibold">Incoming flows (${incoming.length})</h4>
        ${renderSearchGraphEdgeList(incoming, "source")}
      </div>
    </div>
  `;
  details.querySelector("[data-preview-spec]")?.addEventListener("click", (event) => {
    const button = event.currentTarget as HTMLElement;
    emit("openSpecPreview", button.dataset.previewSpec || "");
  });
  details.querySelectorAll<HTMLElement>("[data-select-search-node]").forEach((button) => {
    button.addEventListener("click", () => selectSearchGraphNode(name, button.dataset.selectSearchNode || ""));
  });
}

function codeGraphNodeLabel(result: SearchResult, fileName: string): string {
  const title = result.title || result.nodeId || result.id || fileName || "code";
  return !fileName || title.includes(fileName) ? title : `${title} · ${fileName}`;
}

function renderSearchGraphEdgeList(edges: NetworkGraphLink[], side: "source" | "target"): string {
  if (!edges.length) return '<div class="text-xs text-base-content/50">None</div>';
  return `
    <div class="grid gap-1">
      ${edges
        .slice(0, 10)
        .map((edge) => {
          const related = endpointID(edge[side]);
          return `<button class="graph-ref-row" type="button" data-select-search-node="${escapeHTML(related)}">
            <span class="badge badge-ghost badge-xs">${escapeHTML(edge.type || "references")}</span>
            <span class="min-w-0 truncate">${escapeHTML(related)}</span>
          </button>`;
        })
        .join("")}
    </div>
  `;
}

function searchNodeColor(node: NetworkGraphNode): string {
  switch (node.type) {
    case "code":
      return "#2563eb";
    case "file":
      return "#64748b";
    case "doc":
    case "doc-file":
      return "#0f766e";
    case "flow":
      return "#9333ea";
    default:
      return "#94a3b8";
  }
}

function searchEdgeColorForTheme(currentTheme: "light" | "dark"): (type: string) => string {
  return currentTheme === "dark" ? darkSearchEdgeColor : searchEdgeColor;
}

function searchEdgeColor(type: string): string {
  switch (type) {
    case "defines":
    case "documents":
      return "#64748b";
    case "depends":
    case "blocked-by":
      return "#ef4444";
    case "implements":
    case "calls":
      return "#6366f1";
    case "references":
      return "#14b8a6";
    default:
      return "#64748b";
  }
}

function darkSearchEdgeColor(type: string): string {
  switch (type) {
    case "defines":
    case "documents":
      return "#94a3b8";
    case "depends":
    case "blocked-by":
      return "#f87171";
    case "implements":
    case "calls":
      return "#818cf8";
    case "references":
      return "#2dd4bf";
    default:
      return "#94a3b8";
  }
}

function endpointID(endpoint: string | NetworkGraphNode): string {
  return typeof endpoint === "string" ? endpoint : endpoint?.id || "";
}

function searchGraphDetailsElement(name: string): HTMLElement | null {
  return name === "docsGraph" ? docsGraphDetails.value : codeGraphDetails.value;
}

function stopSearchGraph(name: string) {
  searchGraphRenderers.get(name)?.kill();
  searchGraphRenderers.delete(name);
}

function stopAllSearchGraphs() {
  stopSearchGraph("docsGraph");
  stopSearchGraph("codeGraph");
}

function refitSearchGraphSoon(name: string) {
  window.setTimeout(() => searchGraphRenderers.get(name)?.fit(), 60);
}

function fitSearchGraph(name: string) {
  searchGraphRenderers.get(name)?.fit();
}

function toggleSearchGraphFullscreen(name: string) {
  fullscreenSearchGraph.value = fullscreenSearchGraph.value === name ? "" : name;
  refitSearchGraphSoon(name);
}

function escapeHTML(str: string): string {
  return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#039;");
}

onUnmounted(() => {
  stopAllSearchGraphs();
});
</script>

<template>
  <div class="search-shell border-base-300 bg-base-100 border">
    <div class="search-toolbar border-base-300 border-b">
      <label class="input input-bordered flex min-h-11 flex-1 items-center gap-2">
        <Icon name="search" class="text-base-content/50 h-4 w-4" />
        <input v-model="searchQuery" class="grow" placeholder="Search docs, code, graph nodes; separate keywords with commas" />
      </label>
      <select
        v-model="keywordOperator"
        class="select select-bordered min-h-11"
        aria-label="Keyword result operator"
        title="Keyword result operator"
      >
        <option value="sum">Sum keywords</option>
        <option value="difference">Difference keywords</option>
      </select>
    </div>
    <div id="searchSummary" class="search-summary border-base-300 border-b" v-html="renderSearchSummary()"></div>
    <div class="search-grid">
      <section class="search-panel" data-search-panel="docsSemantic">
        <div class="search-panel-heading">
          <h2>Docs Semantic</h2>
          <span class="badge badge-ghost badge-sm">{{ panelResults("docsSemantic").length }}</span>
        </div>
        <div class="search-results">
          <article v-for="result in panelResults('docsSemantic')" :key="result.id || result.nodeId" class="search-result">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <h3>{{ result.title || result.id || "Untitled" }}</h3>
                <p v-if="result.path" class="search-path">{{ result.path }}</p>
              </div>
              <span class="badge badge-outline badge-sm shrink-0">{{ Math.round((result.score || 0) * 100) }}%</span>
            </div>
            <p v-if="result.description || result.excerpt" class="search-excerpt">{{ result.description || result.excerpt }}</p>
            <div class="mt-2 flex flex-wrap gap-2">
              <button v-if="result.specId" class="btn btn-primary btn-xs" type="button" @click="emit('openSpecPreview', result.specId)">
                <Icon name="file-text" class="h-3.5 w-3.5" />Preview doc
              </button>
            </div>
          </article>
          <div v-if="panelResults('docsSemantic').length === 0 && !searchLoading" class="search-empty">No document semantic matches.</div>
        </div>
      </section>

      <section class="search-panel" data-search-panel="docsGraph">
        <div class="search-panel-heading">
          <h2>Docs Graph</h2>
          <span class="badge badge-ghost badge-sm">{{ panelResults("docsGraph").length }}</span>
        </div>
        <div class="search-results">
          <div
            v-if="panelResults('docsGraph').length > 0"
            class="search-graph-shell"
            :class="{ 'is-fullscreen': fullscreenSearchGraph === 'docsGraph' }"
            data-search-graph-shell="docsGraph"
          >
            <div class="search-graph-toolbar border-base-300 border-b">
              <div class="min-w-0">
                <h3>Docs Graph</h3>
                <p>{{ panelResults("docsGraph").length }} results</p>
              </div>
              <div class="flex items-center gap-2">
                <button
                  class="btn btn-ghost btn-sm"
                  type="button"
                  aria-label="Fit docs graph to center"
                  title="Fit to center"
                  @click="fitSearchGraph('docsGraph')"
                >
                  <Icon name="refresh-cw" class="h-4 w-4" />
                </button>
                <button
                  class="btn btn-ghost btn-sm"
                  type="button"
                  data-fullscreen-graph="docsGraph"
                  :aria-label="fullscreenSearchGraph === 'docsGraph' ? 'Exit full screen docs graph' : 'Full screen docs graph'"
                  :title="fullscreenSearchGraph === 'docsGraph' ? 'Exit full screen' : 'Full screen'"
                  @click="toggleSearchGraphFullscreen('docsGraph')"
                >
                  <Icon :name="fullscreenSearchGraph === 'docsGraph' ? 'minimize' : 'maximize'" class="h-4 w-4" />
                </button>
              </div>
            </div>
            <div class="search-graph-layout">
              <div ref="docsGraphCanvas" class="search-graph-canvas" role="img" aria-label="Docs graph search graph"></div>
              <aside ref="docsGraphDetails" class="search-graph-details"></aside>
            </div>
          </div>
          <div v-if="panelResults('docsGraph').length === 0 && !searchLoading" class="search-empty">No document graph matches.</div>
        </div>
      </section>

      <section class="search-panel" data-search-panel="codeSemantic">
        <div class="search-panel-heading">
          <h2>Code Semantic</h2>
          <span class="badge badge-ghost badge-sm">{{ panelResults("codeSemantic").length }}</span>
        </div>
        <div class="search-results">
          <article v-for="result in panelResults('codeSemantic')" :key="result.id || result.nodeId" class="search-result">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <h3>{{ result.title || result.id || "Untitled" }}</h3>
                <p v-if="result.path" class="search-path">{{ result.path }}</p>
              </div>
              <span class="badge badge-outline badge-sm shrink-0">{{ Math.round((result.score || 0) * 100) }}%</span>
            </div>
            <p v-if="result.description || result.excerpt" class="search-excerpt">{{ result.description || result.excerpt }}</p>
            <div class="mt-2 flex flex-wrap gap-2">
              <button
                v-if="result.path"
                class="btn btn-outline btn-xs"
                type="button"
                @click="emit('openFilePreview', result.path, result.line || 0)"
              >
                <Icon name="file-code" class="h-3.5 w-3.5" />Preview file
              </button>
            </div>
          </article>
          <div v-if="panelResults('codeSemantic').length === 0 && !searchLoading" class="search-empty">No code semantic matches.</div>
        </div>
      </section>

      <section class="search-panel" data-search-panel="codeGraph">
        <div class="search-panel-heading">
          <h2>Code Graph</h2>
          <span class="badge badge-ghost badge-sm">{{ panelResults("codeGraph").length }}</span>
        </div>
        <div class="search-results">
          <div
            v-if="panelResults('codeGraph').length > 0"
            class="search-graph-shell"
            :class="{ 'is-fullscreen': fullscreenSearchGraph === 'codeGraph' }"
            data-search-graph-shell="codeGraph"
          >
            <div class="search-graph-toolbar border-base-300 border-b">
              <div class="min-w-0">
                <h3>Code Graph</h3>
                <p>{{ panelResults("codeGraph").length }} results</p>
              </div>
              <div class="flex items-center gap-2">
                <button
                  class="btn btn-ghost btn-sm"
                  type="button"
                  aria-label="Fit code graph to center"
                  title="Fit to center"
                  @click="fitSearchGraph('codeGraph')"
                >
                  <Icon name="refresh-cw" class="h-4 w-4" />
                </button>
                <button
                  class="btn btn-ghost btn-sm"
                  type="button"
                  data-fullscreen-graph="codeGraph"
                  :aria-label="fullscreenSearchGraph === 'codeGraph' ? 'Exit full screen code graph' : 'Full screen code graph'"
                  :title="fullscreenSearchGraph === 'codeGraph' ? 'Exit full screen' : 'Full screen'"
                  @click="toggleSearchGraphFullscreen('codeGraph')"
                >
                  <Icon :name="fullscreenSearchGraph === 'codeGraph' ? 'minimize' : 'maximize'" class="h-4 w-4" />
                </button>
              </div>
            </div>
            <div class="search-graph-layout">
              <div ref="codeGraphCanvas" class="search-graph-canvas" role="img" aria-label="Code graph search graph"></div>
              <aside ref="codeGraphDetails" class="search-graph-details"></aside>
            </div>
          </div>
          <div v-if="panelResults('codeGraph').length === 0 && !searchLoading" class="search-empty">No code graph matches.</div>
        </div>
      </section>
    </div>
  </div>
</template>
