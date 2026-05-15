type Dictionary<T = unknown> = Record<string, T>;

interface Window {
  hljs?: HighlightGlobal;
  lucide?: LucideGlobal;
  mermaid?: MermaidGlobal;
  svgPanZoom?: SvgPanZoomFactory;
}

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

interface ToastMarkdownViewerConstructor {
  new (options: Dictionary): ToastMarkdownViewer;
}

interface ToastMarkdownViewer {
  destroy(): void;
}

interface ToastMarkdownCodeBlockNode {
  info?: string;
  lang?: string;
  language?: string;
  literal?: string;
}
