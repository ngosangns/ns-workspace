export type MCPTransport = "http" | "sse" | "stdio" | "unknown";

export type MCPServerConfig = Record<string, unknown>;

export function inferTransport(config: unknown): MCPTransport {
  if (!config || typeof config !== "object") return "unknown";
  const c = config as Record<string, unknown>;
  if (c.type === "sse") return "sse";
  if (c.type === "http" || typeof c.url === "string") return "http";
  if (typeof c.command === "string") return "stdio";
  return "unknown";
}

export function transportLabel(t: MCPTransport): string {
  switch (t) {
    case "http":
      return "HTTP";
    case "sse":
      return "SSE";
    case "stdio":
      return "stdio";
    default:
      return "Unknown";
  }
}

/** Short summary line for list/card (truncated ~72 chars). */
export function summarizeConfig(config: unknown, maxLen = 72): string {
  if (!config || typeof config !== "object") return "";
  const c = config as Record<string, unknown>;
  let line = "";
  if (typeof c.url === "string") {
    line = c.url;
  } else if (typeof c.command === "string") {
    const args = Array.isArray(c.args) ? c.args.map(String).join(" ") : "";
    line = args ? `${c.command} ${args}` : c.command;
  } else {
    try {
      line = JSON.stringify(config);
    } catch {
      line = "";
    }
  }
  if (line.length <= maxLen) return line;
  return `${line.slice(0, maxLen - 1)}…`;
}

export function parseArgsLines(text: string): string[] {
  return text
    .split("\n")
    .map((s) => s.trim())
    .filter(Boolean);
}

export function argsToLines(args: unknown): string {
  if (!Array.isArray(args)) return "";
  return args.map(String).join("\n");
}

/** Parse KEY=value lines into a record (empty values kept). */
export function parseEnvLines(text: string): Record<string, string> | undefined {
  const out: Record<string, string> = {};
  for (const line of text.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const eq = trimmed.indexOf("=");
    if (eq <= 0) continue;
    out[trimmed.slice(0, eq).trim()] = trimmed.slice(eq + 1);
  }
  return Object.keys(out).length ? out : undefined;
}

export function envToLines(env: unknown): string {
  if (!env || typeof env !== "object") return "";
  return Object.entries(env as Record<string, unknown>)
    .map(([k, v]) => `${k}=${String(v ?? "")}`)
    .join("\n");
}

export interface MCPFormState {
  name: string;
  transport: "stdio" | "http" | "sse";
  command: string;
  argsText: string;
  envText: string;
  url: string;
  headersText: string;
  /** Vendor keys outside the form schema. */
  extra: Record<string, unknown>;
}

const KNOWN_STDIO = new Set(["command", "args", "env", "type"]);
const KNOWN_HTTP = new Set(["type", "url", "headers"]);

export function configToForm(name: string, config: unknown): MCPFormState {
  const c = config && typeof config === "object" ? ({ ...(config as Record<string, unknown>) } as MCPServerConfig) : {};
  const transport = inferTransport(c);
  const formTransport: MCPFormState["transport"] = transport === "sse" ? "sse" : transport === "http" ? "http" : "stdio";

  const known = formTransport === "stdio" ? KNOWN_STDIO : KNOWN_HTTP;
  const extra: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(c)) {
    if (!known.has(k)) extra[k] = v;
  }

  return {
    name,
    transport: formTransport,
    command: typeof c.command === "string" ? c.command : "",
    argsText: argsToLines(c.args),
    envText: envToLines(c.env),
    url: typeof c.url === "string" ? c.url : "",
    headersText: envToLines(c.headers),
    extra,
  };
}

export function emptyForm(transport: MCPFormState["transport"] = "stdio"): MCPFormState {
  return {
    name: "",
    transport,
    command: "",
    argsText: "",
    envText: "",
    url: "",
    headersText: "",
    extra: {},
  };
}

/** Build config object from form; omits empty optional fields. */
export function formToConfig(form: MCPFormState): MCPServerConfig {
  const extra = { ...form.extra };
  if (form.transport === "stdio") {
    const cfg: MCPServerConfig = { ...extra };
    delete cfg.type;
    cfg.command = form.command.trim();
    const args = parseArgsLines(form.argsText);
    if (args.length) cfg.args = args;
    else delete cfg.args;
    const env = parseEnvLines(form.envText);
    if (env) cfg.env = env;
    else delete cfg.env;
    return cfg;
  }
  const cfg: MCPServerConfig = { ...extra, type: form.transport, url: form.url.trim() };
  delete cfg.command;
  delete cfg.args;
  delete cfg.env;
  const headers = parseEnvLines(form.headersText);
  if (headers) cfg.headers = headers;
  else delete cfg.headers;
  return cfg;
}

export function validateForm(form: MCPFormState, opts?: { requireName?: boolean }): string | null {
  if (opts?.requireName !== false) {
    const name = form.name.trim();
    if (!name) return "Name is required";
    if (/\s/.test(name)) return "Name must not contain spaces";
  }
  if (form.transport === "stdio") {
    if (!form.command.trim()) return "Command is required";
  } else if (!form.url.trim()) {
    return "URL is required";
  }
  return null;
}
