type DiagramKind = "mermaid" | "likec4";

interface DiagramSource {
  kind: DiagramKind;
  source: string;
  title: string;
}

interface LikeC4Node {
  id: string;
  path: string;
  mermaidId: string;
  kind: "softwareSystem" | "container" | "component";
  title: string;
  description: string;
  children: LikeC4Node[];
}

interface LikeC4Relation {
  from: string;
  to: string;
  label: string;
}

const panZoomInstances = new Map<string, SvgPanZoomInstance>();
let diagramSerial = 0;

const diagramSanitizeConfig = {
  USE_PROFILES: { html: true, svg: true, svgFilters: true },
  ADD_TAGS: ["foreignObject", "marker", "defs", "text", "tspan", "div", "span", "p", "br"],
  ADD_ATTR: [
    "viewBox",
    "xmlns",
    "d",
    "x",
    "y",
    "x1",
    "x2",
    "y1",
    "y2",
    "cx",
    "cy",
    "rx",
    "ry",
    "r",
    "points",
    "marker-end",
    "marker-start",
    "text-anchor",
    "dominant-baseline",
    "transform",
    "width",
    "height",
    "fill",
    "stroke",
    "stroke-width",
    "class",
    "id",
    "style",
    "alignment-baseline",
  ],
};

export async function renderDiagramsIn(root: HTMLElement, theme: "light" | "dark", sourceKey = "doc"): Promise<void> {
  destroyDiagramsIn(root);
  const targets = collectDiagramTargets(root);
  for (const [index, target] of targets.entries()) {
    const diagram = diagramSourceFromTarget(target);
    if (!diagram?.source.trim()) continue;

    const host = document.createElement("div");
    const id = `preview-diagram-${safeId(sourceKey)}-${index}-${++diagramSerial}`;
    target.replaceWith(host);
    await renderDiagram(host, id, diagram, theme);
  }
}

export function destroyDiagramsIn(root: HTMLElement): void {
  root.querySelectorAll<HTMLElement>(".diagram-surface[data-diagram-id]").forEach((node) => {
    const id = node.dataset.diagramId;
    if (!id) return;
    panZoomInstances.get(id)?.destroy();
    panZoomInstances.delete(id);
  });
}

function collectDiagramTargets(root: HTMLElement): Element[] {
  const customTags = [...root.querySelectorAll("doc-diagram, doc-graph")];
  const codeBlocks = [...root.querySelectorAll("pre > code")].filter((block) => Boolean(diagramSourceFromCodeBlock(block)));
  return [...customTags, ...codeBlocks.map((block) => block.closest("pre") || block)];
}

function diagramSourceFromTarget(target: Element): DiagramSource | null {
  if (target.matches("doc-diagram, doc-graph")) {
    const type = target.getAttribute("type") || target.getAttribute("language") || "mermaid";
    return diagramSourceFromLanguage(type, target.textContent || "", "Diagram");
  }

  const code = target.matches("pre") ? target.querySelector("code") : target;
  return code ? diagramSourceFromCodeBlock(code) : null;
}

function diagramSourceFromCodeBlock(block: Element): DiagramSource | null {
  const language = blockLanguage(block);
  const source = block.textContent || "";
  if (looksLikeLikeC4Model(source) || language === "likec4" || language === "c4model") {
    return { kind: "likec4", source, title: "LikeC4 model" };
  }
  if (language === "mermaid" || looksLikeMermaidC4(source)) {
    return { kind: "mermaid", source, title: "Mermaid diagram" };
  }

  const c4Type = mermaidC4TypeFromLanguage(language);
  if (c4Type) {
    return {
      kind: "mermaid",
      source: looksLikeMermaidC4(source) ? source : `${c4Type}\n${source}`,
      title: "Mermaid C4 diagram",
    };
  }
  return null;
}

function diagramSourceFromLanguage(language: string, source: string, title: string): DiagramSource | null {
  const normalized = normalizeLanguage(language);
  if (normalized === "likec4" || normalized === "c4model") return { kind: "likec4", source, title: "LikeC4 model" };
  const c4Type = mermaidC4TypeFromLanguage(normalized);
  if (c4Type) return { kind: "mermaid", source: looksLikeMermaidC4(source) ? source : `${c4Type}\n${source}`, title };
  return { kind: "mermaid", source, title };
}

function blockLanguage(block: Element): string {
  const languages = [
    (block as HTMLElement).dataset.sourceLanguage,
    (block as HTMLElement).dataset.language,
    block.getAttribute("data-language"),
    ...block.classList,
  ];
  return normalizeLanguage(languages.find(Boolean) || "");
}

function normalizeLanguage(language: string): string {
  return String(language || "")
    .replace(/^language-/i, "")
    .replace(/[-_\s]/g, "")
    .toLowerCase();
}

function mermaidC4TypeFromLanguage(language: string): string {
  if (!/^c4(?:context|container|component|dynamic|deployment)?$/.test(language)) return "";
  if (language === "c4") return "C4Component";
  const c4Class = language.slice(2);
  return `C4${c4Class.slice(0, 1).toUpperCase()}${c4Class.slice(1)}`;
}

function looksLikeMermaidC4(source: string): boolean {
  return /^\s*C4(?:Context|Container|Component|Dynamic|Deployment)\b/.test(source);
}

function looksLikeLikeC4Model(source: string): boolean {
  return /\bmodel\s*\{/.test(source) && /\b(softwareSystem|container|component)\b/.test(source);
}

async function renderDiagram(host: HTMLElement, id: string, diagram: DiagramSource, theme: "light" | "dark"): Promise<void> {
  const label = diagram.kind === "likec4" ? "LikeC4" : "Mermaid";
  host.className = "mermaid diagram-surface my-5 rounded-lg border border-base-300 bg-base-100";
  host.dataset.diagramId = id;
  host.textContent = "Rendering diagram...";
  try {
    if (!window.mermaid) throw new Error("Mermaid library is not loaded");
    const source = diagram.kind === "likec4" ? likeC4ModelToMermaid(diagram.source) : diagram.source;
    window.mermaid.initialize({
      startOnLoad: false,
      securityLevel: "strict",
      theme: theme === "dark" ? "dark" : "default",
    });
    const result = await window.mermaid.render(id, source);
    host.innerHTML = DOMPurify.sanitize(result.svg || "", diagramSanitizeConfig);
    decorateDiagram(host, id, diagram.title);
  } catch (error) {
    host.className = "alert alert-error my-2 text-sm";
    host.textContent = `${label} render failed: ${error instanceof Error ? error.message : String(error)}`;
  }
}

function decorateDiagram(host: HTMLElement, id: string, title: string): void {
  const svg = host.querySelector<SVGElement>("svg");
  if (!svg) return;
  svg.classList.add("diagram-svg");
  // Mermaid can emit an inline max-width that overrides the preview stylesheet.
  svg.style.maxWidth = "none";
  const toolbar = document.createElement("div");
  toolbar.className = "diagram-toolbar";
  toolbar.innerHTML = `
    <span class="text-base-content/70 truncate text-xs font-semibold">${escapeHTML(title)}</span>
    <span class="flex items-center gap-1">
      <button class="btn btn-ghost btn-xs" type="button" data-diagram-action="zoom-out" aria-label="Zoom out">-</button>
      <span class="diagram-zoom-level text-base-content/60 w-12 text-center text-xs tabular-nums">100%</span>
      <button class="btn btn-ghost btn-xs" type="button" data-diagram-action="zoom-in" aria-label="Zoom in">+</button>
      <button class="btn btn-ghost btn-xs" type="button" data-diagram-action="fit" aria-label="Fit diagram">Fit</button>
      <button class="btn btn-ghost btn-xs" type="button" data-diagram-action="reset" aria-label="Reset zoom">Reset</button>
    </span>`;
  const viewport = document.createElement("div");
  viewport.className = "diagram-viewport";
  viewport.setAttribute("aria-label", `${title}. Ctrl/Command-scroll to zoom, drag to pan.`);
  host.replaceChildren(toolbar, viewport);
  viewport.append(svg);

  if (!window.svgPanZoom) return;
  const zoomLevel = toolbar.querySelector<HTMLElement>(".diagram-zoom-level");
  let instance: SvgPanZoomInstance | null = null;
  instance = window.svgPanZoom(svg, {
    controlIconsEnabled: false,
    fit: true,
    center: true,
    minZoom: 0.25,
    maxZoom: 8,
    mouseWheelZoomEnabled: false,
    zoomScaleSensitivity: 0.25,
    onZoom() {
      if (zoomLevel && instance) zoomLevel.textContent = `${Math.round(instance.getZoom() * 100)}%`;
    },
  });
  panZoomInstances.set(id, instance);
  viewport.addEventListener(
    "wheel",
    (event) => {
      if (!instance || (!event.ctrlKey && !event.metaKey)) return;
      event.preventDefault();
      const rootSvg = svg instanceof SVGSVGElement ? svg : svg.ownerSVGElement;
      if (!rootSvg) return;
      const point = rootSvg.createSVGPoint();
      point.x = event.clientX;
      point.y = event.clientY;
      const screenCTM = rootSvg.getScreenCTM();
      const zoomPoint = screenCTM ? point.matrixTransform(screenCTM.inverse()) : point;
      instance.zoomAtPointBy(event.deltaY < 0 ? 1.25 : 0.8, zoomPoint);
      if (zoomLevel) zoomLevel.textContent = `${Math.round(instance.getZoom() * 100)}%`;
    },
    { passive: false },
  );
  toolbar.querySelector('[data-diagram-action="zoom-in"]')?.addEventListener("click", () => instance.zoomIn());
  toolbar.querySelector('[data-diagram-action="zoom-out"]')?.addEventListener("click", () => instance.zoomOut());
  toolbar.querySelector('[data-diagram-action="fit"]')?.addEventListener("click", () => {
    instance.resize();
    instance.fit();
    instance.center();
  });
  toolbar.querySelector('[data-diagram-action="reset"]')?.addEventListener("click", () => {
    instance.resetZoom();
    instance.resetPan();
    instance.fit();
    instance.center();
  });
}

// The preview supports the architecture-model subset used in docs and converts
// it into Mermaid C4 so the web preview does not need a second diagram runtime.
function likeC4ModelToMermaid(source: string): string {
  const parsed = parseLikeC4Model(source);
  if (!parsed.nodes.length) throw new Error("No C4 nodes found in LikeC4 model");
  const nodeByPath = new Map(parsed.nodes.map((node) => [node.path, node]));
  const nodeByLeaf = new Map(parsed.nodes.map((node) => [node.id, node]));
  const lines = ["C4Component", "title LikeC4 model"];
  parsed.roots.forEach((node) => appendLikeC4Root(lines, node));
  parsed.relations.forEach((relation) => {
    const from = resolveLikeC4Endpoint(relation.from, nodeByPath, nodeByLeaf);
    const to = resolveLikeC4Endpoint(relation.to, nodeByPath, nodeByLeaf);
    lines.push(`Rel(${from}, ${to}, "${escapeMermaidText(relation.label || "uses")}")`);
  });
  return lines.join("\n");
}

function parseLikeC4Model(source: string): { roots: LikeC4Node[]; nodes: LikeC4Node[]; relations: LikeC4Relation[] } {
  const roots: LikeC4Node[] = [];
  const nodes: LikeC4Node[] = [];
  const relations: LikeC4Relation[] = [];
  const stack: LikeC4Node[] = [];

  String(source || "")
    .split("\n")
    .forEach((line) => {
      const trimmed = line.trim();
      if (!trimmed || trimmed === "model {" || trimmed.startsWith("//")) return;
      if (trimmed === "}") {
        stack.pop();
        return;
      }

      const declaration = trimmed.match(/^([A-Za-z0-9_.-]+)\s*=\s*(softwareSystem|container|component)\s+"([^"]+)"\s*(\{)?/);
      if (declaration) {
        const parent = stack.at(-1);
        const path = parent ? `${parent.path}.${declaration[1]}` : declaration[1];
        const node: LikeC4Node = {
          id: declaration[1],
          path,
          mermaidId: likeC4MermaidId(path),
          kind: declaration[2] as LikeC4Node["kind"],
          title: declaration[3],
          description: "",
          children: [],
        };
        if (parent) parent.children.push(node);
        else roots.push(node);
        nodes.push(node);
        if (declaration[4]) stack.push(node);
        return;
      }

      const description = trimmed.match(/^description\s+"([^"]+)"/);
      if (description && stack.length) {
        stack[stack.length - 1].description = description[1];
        return;
      }

      const relation = trimmed.match(/^([A-Za-z0-9_.-]+)\s*->\s*([A-Za-z0-9_.-]+)(?:\s+"([^"]+)")?/);
      if (relation) {
        relations.push({ from: relation[1], to: relation[2], label: relation[3] || "uses" });
      }
    });

  return { roots, nodes, relations };
}

function appendLikeC4Root(lines: string[], node: LikeC4Node): void {
  if (node.kind === "softwareSystem" && node.children.length) {
    node.children.forEach((child) => appendLikeC4Node(lines, child, 0));
    return;
  }
  appendLikeC4Node(lines, node, 0);
}

function appendLikeC4Node(lines: string[], node: LikeC4Node, depth: number): void {
  const pad = "  ".repeat(depth);
  const title = escapeMermaidText(node.title);
  const description = escapeMermaidText(node.description);
  if (node.children.length) {
    const boundary = node.kind === "container" ? "Container_Boundary" : "System_Boundary";
    lines.push(`${pad}${boundary}(${node.mermaidId}, "${title}") {`);
    node.children.forEach((child) => appendLikeC4Node(lines, child, depth + 1));
    lines.push(`${pad}}`);
    return;
  }
  if (node.kind === "component") {
    lines.push(`${pad}Component(${node.mermaidId}, "${title}", "Component", "${description}")`);
    return;
  }
  const type = node.kind === "container" ? "Container" : "System";
  lines.push(`${pad}${type}(${node.mermaidId}, "${title}", "${type}", "${description}")`);
}

function resolveLikeC4Endpoint(endpoint: string, nodeByPath: Map<string, LikeC4Node>, nodeByLeaf: Map<string, LikeC4Node>): string {
  return nodeByPath.get(endpoint)?.mermaidId || nodeByLeaf.get(endpoint)?.mermaidId || likeC4MermaidId(endpoint);
}

function likeC4MermaidId(value: string): string {
  const id = String(value || "")
    .replace(/[^A-Za-z0-9_]+/g, "_")
    .replace(/^_+|_+$/g, "");
  return /^[A-Za-z_]/.test(id) ? id || "likec4_node" : `likec4_${id}`;
}

function escapeMermaidText(value: string): string {
  return String(value || "")
    .replace(/\\/g, "\\\\")
    .replace(/"/g, '\\"');
}

import { escapeHTML } from "./shared-utils.js";

function safeId(value: string): string {
  return String(value || "doc").replace(/[^a-zA-Z0-9_-]/g, "-");
}
