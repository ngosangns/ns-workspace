export function escapeHTML(str: string): string {
  return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#039;");
}

export function endpointID(endpoint: string | { id?: string }): string {
  if (typeof endpoint === "string") return endpoint;
  return endpoint?.id || "";
}

export async function fetchJSON(path: string, signal?: AbortSignal): Promise<any> {
  const res = await fetch(path, { signal });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export function languageFromPath(path: string): string {
  const ext = path.split(".").pop()?.toLowerCase() || "";
  if (ext === "md" || ext === "markdown") return "markdown";
  if (ext === "html" || ext === "htm") return "html";
  return "text";
}
