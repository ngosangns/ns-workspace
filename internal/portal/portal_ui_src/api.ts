const API_BASE = "/api";

export interface Skill {
  id: string;
  name: string;
  description?: string;
  source: string;
  overridden: boolean;
  enabled: boolean;
  content?: string;
}

export interface SkillUpdate {
  content: string;
}

export interface EnableRequest {
  enabled: boolean;
}

export interface MCPServers {
  mcpServers: Record<string, any>;
}

export interface MCPServerItem {
  name: string;
  enabled: boolean;
  config: any;
}

export interface MCPManifest extends MCPServers {
  source: "embedded" | "overlay";
  overridden: boolean;
  disabledServers?: Record<string, any>;
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
  resetMCPs: () => fetchJSON<MCPManifest>("/mcps", { method: "DELETE" }),
  setMCPEnabled: (name: string, enabled: boolean) =>
    fetchJSON<MCPManifest>(`/mcps/${encodeURIComponent(name)}/enabled`, {
      method: "POST",
      body: JSON.stringify({ enabled } satisfies EnableRequest),
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
