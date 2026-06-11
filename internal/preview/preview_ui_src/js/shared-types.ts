import type { InjectionKey, Ref } from "vue";

export interface SpecDocument {
  id: string;
  title: string;
  path: string;
  raw?: string;
  language?: string;
  status?: string;
  version?: string;
  compliance?: string;
  priority?: string;
  description?: string;
}

export interface GraphNode {
  id: string;
  label?: string;
  type?: string;
  path?: string;
  specId?: string;
  category?: string;
  status?: string;
}

interface GraphEdge {
  from: string;
  to: string;
  type?: string;
  label?: string;
}

export interface GraphData {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export interface ProjectSummary {
  name: string;
  generatedTitle?: string;
  projectRoot?: string;
  docsRoot?: string;
  totalSpecs: number;
  categories?: Record<string, number>;
  statusCounts?: Record<string, number>;
  compliance?: Record<string, number>;
  warnings?: string[];
  sync?: Record<string, string>;
}

export interface PreviewSource {
  type: "doc" | "file";
  raw: string;
  language: string;
  path: string;
  line: number;
  spec?: SpecDocument;
}

export type ThemePreference = "system" | "dark" | "light";

export const ProjectKey: InjectionKey<Ref<ProjectSummary | null>> = Symbol("project");
export const SpecsKey: InjectionKey<Ref<SpecDocument[]>> = Symbol("specs");
export const GraphKey: InjectionKey<Ref<GraphData | null>> = Symbol("graph");
export const CurrentSpecKey: InjectionKey<Ref<SpecDocument | null>> = Symbol("currentSpec");
export const SelectedIdKey: InjectionKey<Ref<string>> = Symbol("selectedId");
export const SelectedFolderPathKey: InjectionKey<Ref<string>> = Symbol("selectedFolderPath");
export const TabKey: InjectionKey<Ref<string>> = Symbol("tab");
export const ThemeKey: InjectionKey<Ref<"light" | "dark">> = Symbol("theme");
export const SearchQueryKey: InjectionKey<Ref<string>> = Symbol("searchQuery");
export const SearchKeywordOperatorKey: InjectionKey<Ref<string>> = Symbol("searchKeywordOperator");
export const SelectSpecKey: InjectionKey<(id: string, showSpecTab?: boolean) => Promise<void>> = Symbol("selectSpec");
export const OpenSpecPreviewKey: InjectionKey<(id: string) => void> = Symbol("openSpecPreview");
export const OpenFilePreviewKey: InjectionKey<(path: string, line: number) => void> = Symbol("openFilePreview");
export const ClosePreviewKey: InjectionKey<() => void> = Symbol("closePreview");
export const ToggleThemeKey: InjectionKey<() => void> = Symbol("toggleTheme");
