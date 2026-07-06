const API_BASE = "/api";

export interface Skill {
  id: string;
  name: string;
  source: string;
  overridden: boolean;
  content?: string;
}

export interface SkillUpdate {
  content: string;
}

export interface MCPServers {
  mcpServers: Record<string, any>;
}

export interface MCPManifest extends MCPServers {
  source: "embedded" | "overlay";
  overridden: boolean;
}

export interface ClaudeEnv {
  ANTHROPIC_BASE_URL?: string;
  ANTHROPIC_AUTH_TOKEN?: string;
  ANTHROPIC_MODEL?: string;
  ANTHROPIC_SMALL_FAST_MODEL?: string;
}

export interface ClaudeSettings {
  permissions: Record<string, any>;
  env: ClaudeEnv;
  source: "embedded" | "overlay";
  overridden: boolean;
}

export interface RegistrySkill {
  name: string;
  source: string;
  skill: string;
}

export interface RegistrySkills {
  skills: RegistrySkill[];
}

export interface Adapter {
  id: string;
  name: string;
  tier: string;
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
  dryRun: boolean;
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

  getMCPs: () => fetchJSON<MCPManifest>("/mcps"),
  getMCPPreset: () => fetchJSON<MCPServers>("/mcps/preset"),
  updateMCPs: (servers: MCPServers) =>
    fetchJSON<MCPManifest>("/mcps", {
      method: "PUT",
      body: JSON.stringify(servers),
    }),
  resetMCPs: () => fetchJSON<MCPManifest>("/mcps", { method: "DELETE" }),

  getClaudeSettings: () => fetchJSON<ClaudeSettings>("/settings/claude"),
  getClaudeSettingsPreset: () => fetchJSON<ClaudeSettings>("/settings/claude/preset"),
  updateClaudeSettings: (settings: ClaudeSettings) =>
    fetchJSON<ClaudeSettings>("/settings/claude", {
      method: "PUT",
      body: JSON.stringify(settings),
    }),
  resetClaudeSettings: () => fetchJSON<ClaudeSettings>("/settings/claude", { method: "DELETE" }),

  getRegistry: () => fetchJSON<RegistrySkills>("/registry"),
  updateRegistry: (registry: RegistrySkills) =>
    fetchJSON<RegistrySkills>("/registry", {
      method: "PUT",
      body: JSON.stringify(registry),
    }),

  getAdapters: () => fetchJSON<Adapter[]>("/adapters"),

  getStatus: () => fetchJSON<StatusSummary>("/status"),
  getConfig: () => fetchJSON<UserOverlay>("/config"),

  startSync: (command: string, dryRun = false) =>
    fetchJSON<SyncJob>(`/sync/${command}`, {
      method: "POST",
      body: JSON.stringify({ command, dryRun }),
    }),
  streamSync: (jobId: string) => new EventSource(`${API_BASE}/sync/stream?jobId=${encodeURIComponent(jobId)}`),
};
