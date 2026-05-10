import Graph from "graphology";
import forceAtlas2 from "graphology-layout-forceatlas2";
import Sigma from "sigma";

export interface NetworkGraphNode {
  id: string;
  label?: string;
  type?: string;
  path?: string;
  specId?: string;
  line?: number;
  [key: string]: any;
}

export interface NetworkGraphLink {
  source: string | NetworkGraphNode;
  target: string | NetworkGraphNode;
  type?: string;
  label?: string;
  confidence?: string;
  origin?: string;
  [key: string]: any;
}

export interface NetworkGraphData {
  nodes: NetworkGraphNode[];
  links: NetworkGraphLink[];
}

export interface NetworkGraphRenderer {
  fit(): void;
  kill(): void;
  setSelected(id: string): void;
}

interface RenderNetworkGraphOptions {
  container: HTMLElement;
  graph: NetworkGraphData;
  selectedId: string;
  nodeColor: (node: NetworkGraphNode) => string;
  edgeColor: (type: string) => string;
  onSelectNode: (node: NetworkGraphNode) => void;
  labelColor?: string;
}

interface SigmaNodeAttributes extends NetworkGraphNode {
  x: number;
  y: number;
  size: number;
  color: string;
  label: string;
}

interface SigmaEdgeAttributes {
  color: string;
  label: string;
  size: number;
  type: string;
  relation: string;
  confidence: string;
}

export function renderNetworkGraph(options: RenderNetworkGraphOptions): NetworkGraphRenderer {
  const graph = new Graph<SigmaNodeAttributes, SigmaEdgeAttributes>({ multi: true, type: "directed" });
  const nodes = normalizeNodes(options.graph.nodes);

  seedCircularLayout(nodes);
  for (const node of nodes) {
    graph.mergeNode(node.id, {
      ...node,
      color: options.nodeColor(node),
      label: node.label || node.id,
      size: nodeSize(node),
    });
  }

  addEdges(graph, options.graph.links, options.edgeColor);
  applyReadableLayout(graph);

  let selectedId = options.selectedId && graph.hasNode(options.selectedId) ? options.selectedId : nodes[0]?.id || "";
  const renderer = new Sigma<SigmaNodeAttributes, SigmaEdgeAttributes>(graph, options.container, {
    allowInvalidContainer: true,
    autoCenter: true,
    autoRescale: true,
    defaultEdgeType: "arrow",
    defaultNodeColor: "#94a3b8",
    hideEdgesOnMove: false,
    hideLabelsOnMove: false,
    itemSizesReference: "screen",
    labelColor: { color: options.labelColor || "#0f172a" },
    labelDensity: 1,
    labelGridCellSize: 90,
    labelRenderedSizeThreshold: 0,
    labelSize: 12,
    labelWeight: "650",
    maxCameraRatio: 10,
    minCameraRatio: 0.05,
    minEdgeThickness: 1.3,
    renderEdgeLabels: false,
    renderLabels: true,
    zIndex: true,
    nodeReducer: (node, data) => {
      const selected = Boolean(selectedId);
      const related = selected && (node === selectedId || graph.areNeighbors(node, selectedId));
      return {
        ...data,
        color: selected && !related ? softenColor(data.color) : data.color,
        forceLabel: true,
        highlighted: node === selectedId,
        label: data.label,
        size: node === selectedId ? data.size + 3 : related ? data.size + 1.5 : data.size,
        type: "circle",
        zIndex: node === selectedId ? 3 : related ? 2 : 1,
      };
    },
    edgeReducer: (edge, data) => {
      const [source, target] = graph.extremities(edge);
      const related = Boolean(selectedId) && (source === selectedId || target === selectedId);
      return {
        ...data,
        color: related ? data.color : softenColor(data.color),
        hidden: Boolean(selectedId) && graph.order > 180 && !related,
        size: related ? data.size + 0.8 : data.size,
        zIndex: related ? 2 : 1,
      };
    },
  });

  renderer.on("clickNode", ({ node }) => {
    const attrs = graph.getNodeAttributes(node);
    selectedId = node;
    renderer.refresh();
    options.onSelectNode(attrs);
  });
  renderer.on("enterNode", () => {
    options.container.classList.add("is-node-hover");
  });
  renderer.on("leaveNode", () => {
    options.container.classList.remove("is-node-hover");
  });

  requestAnimationFrame(() => fitRenderer(renderer));

  return {
    fit: () => fitRenderer(renderer),
    kill: () => renderer.kill(),
    setSelected: (id: string) => {
      selectedId = graph.hasNode(id) ? id : "";
      renderer.refresh();
    },
  };
}

function normalizeNodes(nodes: NetworkGraphNode[]): SigmaNodeAttributes[] {
  const out: SigmaNodeAttributes[] = [];
  const seen = new Set<string>();
  for (const node of nodes || []) {
    const id = String(node.id || "").trim();
    if (!id || seen.has(id)) continue;
    seen.add(id);
    out.push({
      ...node,
      id,
      label: node.label || id,
      x: 0,
      y: 0,
      size: nodeSize(node),
      color: "#94a3b8",
    });
  }
  return out;
}

function seedCircularLayout(nodes: SigmaNodeAttributes[]) {
  const radius = Math.max(8, Math.sqrt(Math.max(nodes.length, 1)) * 9);
  nodes.forEach((node, index) => {
    const angle = (index / Math.max(nodes.length, 1)) * Math.PI * 2;
    const jitter = (stableHash(node.id) % 17) / 17;
    node.x = Math.cos(angle) * (radius + jitter * 2);
    node.y = Math.sin(angle) * (radius + jitter * 2);
  });
}

function addEdges(graph: Graph<SigmaNodeAttributes, SigmaEdgeAttributes>, links: NetworkGraphLink[], edgeColor: (type: string) => string) {
  let serial = 0;
  for (const link of links || []) {
    const source = endpointID(link.source);
    const target = endpointID(link.target);
    if (!source || !target || source === target || !graph.hasNode(source) || !graph.hasNode(target)) continue;
    const relation = link.type || link.label || "references";
    const key = `edge:${source}:${target}:${relation}:${serial++}`;
    graph.addDirectedEdgeWithKey(key, source, target, {
      color: edgeColor(relation),
      confidence: link.confidence || link.origin || "",
      label: relation,
      relation,
      size: relation === "defines" || relation === "documents" ? 1.2 : 1.5,
      type: "arrow",
    });
  }
}

function applyReadableLayout(graph: Graph<SigmaNodeAttributes, SigmaEdgeAttributes>) {
  if (graph.order < 2) return;
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

function fitRenderer(renderer: Sigma<SigmaNodeAttributes, SigmaEdgeAttributes>) {
  renderer.resize(true);
  renderer.refresh();
  renderer
    .getCamera()
    .animatedReset({ duration: 260 })
    .then(() => {
      renderer.resize(true);
      renderer.refresh();
    })
    .catch(() => {});
  requestAnimationFrame(() => {
    renderer.refresh();
    requestAnimationFrame(() => renderer.refresh());
  });
}

function endpointID(endpoint: string | NetworkGraphNode): string {
  if (typeof endpoint === "string") return endpoint;
  return endpoint?.id || "";
}

function nodeSize(node: NetworkGraphNode): number {
  switch (node.type) {
    case "external":
    case "file":
    case "doc-file":
      return 6;
    case "flow":
      return 7;
    default:
      return 8;
  }
}

function softenColor(color: string): string {
  const hex = color.startsWith("#") ? color.slice(1) : color;
  if (hex.length !== 6) return "#cbd5e1";
  const r = Number.parseInt(hex.slice(0, 2), 16);
  const g = Number.parseInt(hex.slice(2, 4), 16);
  const b = Number.parseInt(hex.slice(4, 6), 16);
  return `rgb(${Math.round((r + 226) / 2)}, ${Math.round((g + 232) / 2)}, ${Math.round((b + 240) / 2)})`;
}

function stableHash(value: string): number {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
  }
  return hash;
}
