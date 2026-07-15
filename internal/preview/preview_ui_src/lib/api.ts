const API_BASE = "/api";

export interface ProjectSummary {
  name: string;
  projectRoot: string;
  docsRoot: string;
  totalSpecs: number;
  generatedTitle?: string;
  categories?: Record<string, number>;
  warnings?: string[];
}

export interface SpecDocument {
  id: string;
  title: string;
  path: string;
  category: string;
  status?: string;
  description?: string;
  type?: string;
  tags?: string[];
  html?: string;
  raw?: string;
}

export interface SearchResult {
  id: string;
  title: string;
  path?: string;
  score: number;
  excerpt?: string;
  description?: string;
  kind?: string;
  source?: string;
  line?: number;
}

export interface SearchResponse {
  query: string;
  mode: string;
  panels: {
    docsSemantic: SearchResult[];
    docsGraph: SearchResult[];
    codeSemantic: SearchResult[];
    codeGraph: SearchResult[];
  };
  stats: Record<string, number>;
  warnings?: string[];
}

export interface GraphNode {
  id: string;
  label: string;
  type?: string;
  path?: string;
  category?: string;
}

export interface GraphEdge {
  from: string;
  to: string;
  label?: string;
  type?: string;
}

export interface SpecGraph {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

async function fetchJSON<T>(path: string): Promise<T> {
  const res = await fetch(API_BASE + path);
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error || res.statusText);
  }
  return res.json();
}

export const api = {
  getProject: () => fetchJSON<ProjectSummary>("/project"),
  getDocs: () => fetchJSON<SpecDocument[]>("/docs"),
  getDoc: (id: string) => fetchJSON<SpecDocument>(`/docs/${encodeURIComponent(id)}`),
  search: (q: string, limit = 12) => fetchJSON<SearchResponse>(`/search?q=${encodeURIComponent(q)}&limit=${limit}`),
  getGraph: () => fetchJSON<SpecGraph>("/graph"),
  events: () => new EventSource(`${API_BASE}/events`),
};
