const API_BASE = "/api";

export interface Skill {
  id: string;
  name: string;
  description?: string;
  /** embedded | overlay | installed */
  source: string;
  /** GitHub owner/repo when linked via registry overlay. */
  registrySource?: string;
  overridden: boolean;
  enabled: boolean;
  content?: string;
}

export interface CatalogSkill {
  id: string;
  /** Installable id for `npx skills add --skill` (frontmatter name, not always folder). */
  skillId: string;
  name: string;
  description?: string;
  installs?: number;
  source: string;
  url?: string;
  path?: string;
  installed: boolean;
}

export interface CatalogSearchResponse {
  query: string;
  skills: CatalogSkill[];
}

export interface RegistrySource {
  source: string;
  /** Configured rows in skills.json for this source. */
  enabledEntries: number;
  /** Configured rows in skills.disabled.json for this source. */
  disabledEntries: number;
  /**
   * Installable skills in the package (GitHub SKILL.md count).
   * /skills/registries returns 0; filled after /skills/catalog for that source.
   * Not the same as enabledEntries + disabledEntries (configured rows).
   */
  skillCount: number;
  configured: boolean;
  listable: boolean;
  note?: string;
}

export interface RegistriesResponse {
  registries: RegistrySource[];
}

export interface CatalogListResponse {
  registry?: string;
  query?: string;
  skills: CatalogSkill[];
  count: number;
}

export interface SkillInstallRequest {
  source: string;
  skill: string;
  name?: string;
  /** Repo-relative SKILL.md path from catalog — enables direct install fallback. */
  path?: string;
}

export interface SkillInstallResponse {
  skill: Skill;
  registry: RegistrySkill;
}

export interface SkillInstallBatchResponse {
  installed: SkillInstallResponse[];
  failed?: { source: string; skill: string; error: string }[];
}

export interface SkillUpdate {
  content: string;
}

export interface EnableRequest {
  enabled: boolean;
}

/** Loose MCP server config (stdio / HTTP / SSE / vendor fields). */
export type MCPServerConfig = Record<string, unknown>;

export interface MCPServers {
  mcpServers: Record<string, MCPServerConfig>;
}

export interface MCPServerItem {
  name: string;
  enabled: boolean;
  config: MCPServerConfig;
}

export interface MCPManifest extends MCPServers {
  source: "embedded" | "overlay" | string;
  overridden: boolean;
  disabledServers?: Record<string, MCPServerConfig>;
  items?: MCPServerItem[];
  /**
   * Single catalog document:
   * `{ "mcpServers": { ...all... }, "disabled": ["name"] }`.
   */
  content?: string;
}

export interface RegistrySkill {
  name: string;
  source?: string;
  skill: string;
  installer?: string;
}

export interface RegistrySkillItem extends RegistrySkill {
  enabled: boolean;
}

export interface RegistrySkills {
  skills: RegistrySkill[];
  disabledSkills?: RegistrySkill[];
  items?: RegistrySkillItem[];
  overridden?: boolean;
  source?: string;
}

export interface Adapter {
  id: string;
  name: string;
  tier: string;
  enabled: boolean;
  docs?: string[];
  artifacts?: string[];
  notes?: string;
}

export interface PathStatus {
  path: string;
  exists: boolean;
  isDir: boolean;
}

export interface StatusSummary {
  agentsDir: string;
  paths: PathStatus[];
}

export interface SyncJob {
  id: string;
  command: string;
  running: boolean;
  error?: string;
}

export interface UserOverlay {
  origin: string;
  entries: Record<string, string>;
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(API_BASE + path, {
    headers: { "Content-Type": "application/json" },
    ...init,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new ApiError(res.status, body.error || res.statusText);
  }
  if (res.status === 204) {
    return undefined as T;
  }
  return res.json();
}

export const api = {
  getSkills: () => fetchJSON<Skill[]>("/skills"),
  getSkill: (id: string) => fetchJSON<Skill>(`/skills/${id}`),
  updateSkill: (id: string, content: string) =>
    fetchJSON<Skill>(`/skills/${id}`, {
      method: "PUT",
      body: JSON.stringify({ content }),
    }),
  resetSkill: (id: string) => fetchJSON<void>(`/skills/${id}`, { method: "DELETE" }),
  setSkillEnabled: (id: string, enabled: boolean) =>
    fetchJSON<Skill>(`/skills/${id}/enabled`, {
      method: "POST",
      body: JSON.stringify({ enabled } satisfies EnableRequest),
    }),
  searchSkillsCatalog: (q: string, registry?: string) => {
    const params = new URLSearchParams({ q });
    if (registry) params.set("registry", registry);
    return fetchJSON<CatalogSearchResponse>(`/skills/search?${params}`);
  },
  listSkillRegistries: () => fetchJSON<RegistriesResponse>("/skills/registries"),
  listSkillsCatalog: (opts?: { registry?: string; q?: string; refresh?: boolean }) => {
    const params = new URLSearchParams();
    if (opts?.registry) params.set("registry", opts.registry);
    if (opts?.q) params.set("q", opts.q);
    if (opts?.refresh) params.set("refresh", "1");
    const qs = params.toString();
    return fetchJSON<CatalogListResponse>(`/skills/catalog${qs ? `?${qs}` : ""}`);
  },
  installSkill: (body: SkillInstallRequest) =>
    fetchJSON<SkillInstallResponse>("/skills/install", {
      method: "POST",
      body: JSON.stringify(body),
    }),
  installSkillsBatch: (skills: SkillInstallRequest[]) =>
    fetchJSON<SkillInstallBatchResponse>("/skills/install-batch", {
      method: "POST",
      body: JSON.stringify({ skills }),
    }),
  uninstallSkill: (skill: string) =>
    fetchJSON<{ ok: boolean; skill: string }>("/skills/uninstall", {
      method: "POST",
      body: JSON.stringify({ skill }),
    }),

  getMCPs: () => fetchJSON<MCPManifest>("/mcps"),
  getMCPPreset: () => fetchJSON<MCPServers>("/mcps/preset"),
  updateMCPs: (servers: MCPServers) =>
    fetchJSON<MCPManifest>("/mcps", {
      method: "PUT",
      body: JSON.stringify(servers),
    }),
  updateMCPsContent: (content: string) =>
    fetchJSON<MCPManifest>("/mcps", {
      method: "PUT",
      body: JSON.stringify({ content }),
    }),
  /** Full catalog replace with optional disabled name list. */
  updateMCPCatalog: (mcpServers: Record<string, MCPServerConfig>, disabled?: string[]) =>
    fetchJSON<MCPManifest>("/mcps", {
      method: "PUT",
      body: JSON.stringify({ mcpServers, disabled: disabled ?? [] }),
    }),
  resetMCPs: () => fetchJSON<MCPManifest>("/mcps", { method: "DELETE" }),
  setMCPEnabled: (name: string, enabled: boolean) =>
    fetchJSON<MCPManifest>(`/mcps/${encodeURIComponent(name)}/enabled`, {
      method: "POST",
      body: JSON.stringify({ enabled } satisfies EnableRequest),
    }),
  deleteMCP: (name: string) =>
    fetchJSON<MCPManifest>(`/mcps/${encodeURIComponent(name)}`, {
      method: "DELETE",
    }),

  getRegistry: () => fetchJSON<RegistrySkills>("/registry"),
  updateRegistry: (registry: RegistrySkills) =>
    fetchJSON<RegistrySkills>("/registry", {
      method: "PUT",
      body: JSON.stringify({ skills: registry.skills }),
    }),
  setRegistrySkillEnabled: (name: string, enabled: boolean) =>
    fetchJSON<RegistrySkills>(`/registry/${encodeURIComponent(name)}/enabled`, {
      method: "POST",
      body: JSON.stringify({ enabled } satisfies EnableRequest),
    }),
  resetRegistry: () => fetchJSON<RegistrySkills>("/registry", { method: "DELETE" }),

  getAdapters: () => fetchJSON<Adapter[]>("/adapters"),
  setAdapterEnabled: (id: string, enabled: boolean) =>
    fetchJSON<Adapter>(`/adapters/${encodeURIComponent(id)}/enabled`, {
      method: "POST",
      body: JSON.stringify({ enabled } satisfies EnableRequest),
    }),

  getStatus: () => fetchJSON<StatusSummary>("/status"),
  getConfig: () => fetchJSON<UserOverlay>("/config"),

  startSync: (command: string, tools?: string) =>
    fetchJSON<SyncJob>(`/sync/${command}`, {
      method: "POST",
      body: JSON.stringify({ command, tools }),
    }),
  streamSync: (jobId: string) => new EventSource(`${API_BASE}/sync/stream?jobId=${encodeURIComponent(jobId)}`),
};
