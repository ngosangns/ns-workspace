type Dictionary<T = unknown> = Record<string, T>;

interface Window {
  d3?: D3Global;
  hljs?: HighlightGlobal;
  lucide?: LucideGlobal;
  markdownit?: MarkdownItFactory;
  mermaid?: MermaidGlobal;
  svgPanZoom?: SvgPanZoomFactory;
}

declare const d3: D3Global;
declare const DOMPurify: {
  sanitize(input: string, config?: Dictionary): string;
};
declare const lucide: LucideGlobal;

interface HighlightGlobal {
  getLanguage(language: string): unknown;
  highlight(source: string, options: { language: string; ignoreIllegals: boolean }): { value: string };
  highlightElement(block: Element): void;
}

interface LucideGlobal {
  createIcons(): void;
}

interface MarkdownItFactory {
  (options: { html: boolean; linkify: boolean; typographer: boolean; highlight: (source: string, lang?: string) => string }): {
    enable(rule: string | string[]): void;
    render(raw: string): string;
  };
}

interface MermaidGlobal {
  initialize(options: Dictionary): void;
  render(id: string, source: string): Promise<{ svg?: string }>;
}

interface SvgPanZoomInstance {
  center(): void;
  destroy(): void;
  fit(): void;
  getZoom(): number;
  resetPan(): void;
  resetZoom(): void;
  resize(): void;
  zoomIn(): void;
  zoomOut(): void;
}

interface SvgPanZoomFactory {
  (svg: SVGElement, options: Dictionary): SvgPanZoomInstance;
}

type D3Global = any;
