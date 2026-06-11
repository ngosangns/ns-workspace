import Graph from "graphology";
import forceAtlas2 from "graphology-layout-forceatlas2";
import Sigma from "sigma";

export interface NetworkGraphNode {
  id: string;
  label?: string;
  type?: string;
  path?: string;
  specId?: string;
  isRootCaller?: boolean;
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

interface NetworkGraphRenderer {
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
  onClearSelection?: () => void;
  labelColor?: string;
  rootCallerBorderColor?: string;
  unfocusedEdgeColor?: string;
}

interface SigmaNodeAttributes extends NetworkGraphNode {
  x: number;
  y: number;
  size: number;
  color: string;
  label: string;
  labelColor: string;
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
      labelColor: options.labelColor || "#0f172a",
      size: nodeSize(node),
    });
  }

  addEdges(graph, options.graph.links, options.edgeColor);
  applyReadableLayout(graph);

  let selectedId = options.selectedId && graph.hasNode(options.selectedId) ? options.selectedId : "";
  const renderer = new Sigma<SigmaNodeAttributes, SigmaEdgeAttributes>(graph, options.container, {
    allowInvalidContainer: true,
    autoCenter: true,
    autoRescale: true,
    defaultEdgeType: "arrow",
    defaultNodeColor: "#94a3b8",
    defaultDrawNodeLabel: drawNodeLabel,
    defaultDrawNodeHover: drawNodeLabel,
    hideEdgesOnMove: false,
    hideLabelsOnMove: false,
    itemSizesReference: "screen",
    labelColor: { attribute: "labelColor", color: options.labelColor || "#0f172a" },
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
      const dimmed = selected && !related;
      return {
        ...data,
        color: dimmed ? colorWithOpacity(data.color, 0.18) : data.color,
        forceLabel: true,
        label: data.label,
        labelColor: dimmed ? colorWithOpacity(data.labelColor, 0.22) : data.labelColor,
        type: "circle",
        zIndex: node === selectedId ? 4 : related ? 3 : 1,
      };
    },
    edgeReducer: (edge, data) => {
      const [source, target] = graph.extremities(edge);
      const selected = Boolean(selectedId);
      const related = selected && (source === selectedId || target === selectedId);
      // Dark canvases need a solid muted edge color; alpha-blended relation colors can look brighter than focused paths.
      const unfocusedColor = options.unfocusedEdgeColor || colorWithOpacity(data.color, 0.14);
      return {
        ...data,
        color: selected && !related ? unfocusedColor : data.color,
        hidden: false,
      };
    },
  });
  const disposeWheelGuard = installModifierWheelZoomGuard(options.container);
  const disposeRootCallerBorder = installRootCallerBorderOverlay(
    renderer,
    options.container,
    nodes.filter((node) => node.isRootCaller).map((node) => node.id),
    options.rootCallerBorderColor || "#000000",
  );

  renderer.on("clickNode", ({ node }) => {
    const attrs = graph.getNodeAttributes(node);
    selectedId = node;
    renderer.refresh();
    options.onSelectNode(attrs);
  });
  renderer.on("clickStage", () => {
    if (!selectedId) return;
    selectedId = "";
    renderer.refresh();
    options.onClearSelection?.();
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
    kill: () => {
      disposeWheelGuard();
      disposeRootCallerBorder();
      renderer.kill();
    },
    setSelected: (id: string) => {
      selectedId = graph.hasNode(id) ? id : "";
      renderer.refresh();
    },
  };
}

function installRootCallerBorderOverlay(
  renderer: Sigma<SigmaNodeAttributes, SigmaEdgeAttributes>,
  container: HTMLElement,
  nodeIds: string[],
  borderColor: string,
): () => void {
  const rootNodeIds = [...new Set(nodeIds.filter(Boolean))];
  if (rootNodeIds.length === 0) return () => {};

  const overlay = document.createElement("canvas");
  overlay.className = "network-graph-root-caller-border";
  overlay.style.position = "absolute";
  overlay.style.inset = "0";
  overlay.style.pointerEvents = "none";
  overlay.style.zIndex = "3";
  if (getComputedStyle(container).position === "static") {
    container.style.position = "relative";
  }
  container.appendChild(overlay);

  let frame = 0;
  const draw = () => {
    frame = 0;
    const width = container.clientWidth;
    const height = container.clientHeight;
    const ratio = window.devicePixelRatio || 1;
    const canvasWidth = Math.max(1, Math.round(width * ratio));
    const canvasHeight = Math.max(1, Math.round(height * ratio));
    if (overlay.width !== canvasWidth || overlay.height !== canvasHeight) {
      overlay.width = canvasWidth;
      overlay.height = canvasHeight;
      overlay.style.width = `${width}px`;
      overlay.style.height = `${height}px`;
    }

    const context = overlay.getContext("2d");
    if (!context) return;
    context.clearRect(0, 0, overlay.width, overlay.height);
    context.save();
    context.scale(ratio, ratio);
    context.strokeStyle = borderColor;
    context.lineWidth = 2;
    for (const nodeId of rootNodeIds) {
      const displayData = renderer.getNodeDisplayData(nodeId);
      if (!displayData || !Number.isFinite(displayData.x) || !Number.isFinite(displayData.y)) continue;
      const position = renderer.framedGraphToViewport(displayData);
      const radius = Math.max(8, renderer.scaleSize(Number(displayData.size || 8)) + 2);
      context.beginPath();
      context.arc(position.x, position.y, radius, 0, Math.PI * 2);
      context.stroke();
    }
    context.restore();
  };
  const scheduleDraw = () => {
    if (frame) return;
    frame = requestAnimationFrame(draw);
  };
  const camera = renderer.getCamera();
  camera.on("updated", scheduleDraw);
  window.addEventListener("resize", scheduleDraw);
  renderer.on("afterRender", scheduleDraw);
  requestAnimationFrame(() => {
    scheduleDraw();
    requestAnimationFrame(scheduleDraw);
  });

  return () => {
    if (frame) cancelAnimationFrame(frame);
    camera.removeListener("updated", scheduleDraw);
    window.removeEventListener("resize", scheduleDraw);
    renderer.removeListener("afterRender", scheduleDraw);
    overlay.remove();
  };
}

function installModifierWheelZoomGuard(container: HTMLElement): () => void {
  const options = { capture: true };
  const handleWheel = (event: WheelEvent) => {
    if (event.metaKey || event.ctrlKey) return;
    event.stopPropagation();
    event.stopImmediatePropagation();
  };
  container.addEventListener("wheel", handleWheel, options);
  return () => container.removeEventListener("wheel", handleWheel, options);
}

function drawNodeLabel(context: CanvasRenderingContext2D, data: any, settings: any) {
  if (!data.label) return;
  const size = settings.labelSize;
  const font = settings.labelFont;
  const weight = settings.labelWeight;
  const color = settings.labelColor.attribute
    ? data[settings.labelColor.attribute] || settings.labelColor.color || "#000"
    : settings.labelColor.color;
  context.fillStyle = color;
  context.font = `${weight} ${size}px ${font}`;
  const lines = multilineLabel(data.label);
  const lineHeight = Math.round(size * 1.18);
  const startY = data.y - ((lines.length - 1) * lineHeight) / 2 + size / 3;
  lines.forEach((line, index) => {
    context.fillText(line, data.x + data.size + 3, startY + index * lineHeight);
  });
}

function multilineLabel(label: string): string[] {
  return String(label || "")
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
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
      labelColor: "#0f172a",
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
      size: relation === "calls" || relation === "call" ? 2.1 : relation === "defines" || relation === "documents" ? 1.2 : 1.5,
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

import { endpointID } from "./shared-utils.js";

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

function colorWithOpacity(color: string, opacity: number): string {
  const hex = color.startsWith("#") ? color.slice(1) : "";
  if (hex.length === 3) {
    const r = Number.parseInt(hex[0] + hex[0], 16);
    const g = Number.parseInt(hex[1] + hex[1], 16);
    const b = Number.parseInt(hex[2] + hex[2], 16);
    return `rgba(${r}, ${g}, ${b}, ${opacity})`;
  }
  if (hex.length === 6) {
    const r = Number.parseInt(hex.slice(0, 2), 16);
    const g = Number.parseInt(hex.slice(2, 4), 16);
    const b = Number.parseInt(hex.slice(4, 6), 16);
    return `rgba(${r}, ${g}, ${b}, ${opacity})`;
  }
  const rgb = color.match(/^rgb\((\d+),\s*(\d+),\s*(\d+)\)$/);
  if (rgb) return `rgba(${rgb[1]}, ${rgb[2]}, ${rgb[3]}, ${opacity})`;
  const rgba = color.match(/^rgba\((\d+),\s*(\d+),\s*(\d+),\s*[\d.]+\)$/);
  if (rgba) return `rgba(${rgba[1]}, ${rgba[2]}, ${rgba[3]}, ${opacity})`;
  return color;
}

function stableHash(value: string): number {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
  }
  return hash;
}
