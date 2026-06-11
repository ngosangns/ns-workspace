import type { SpecDocument } from "./shared-types.js";

export type { SpecDocument };

export interface InternalSpecTarget {
  specId: string;
  fragment: string;
}

export function decorateInternalDocNavigation(
  root: HTMLElement,
  spec: SpecDocument,
  specs: SpecDocument[],
  onNavigate: (target: InternalSpecTarget) => void,
): void {
  const lookup = buildSpecLookup(specs);
  decorateInternalDocLinks(root, spec, lookup, onNavigate);
  decorateInternalDocMentions(root, spec, lookup, onNavigate);
}

function decorateInternalDocLinks(
  root: HTMLElement,
  spec: SpecDocument,
  lookup: Map<string, SpecDocument>,
  onNavigate: (target: InternalSpecTarget) => void,
): void {
  root.querySelectorAll<HTMLAnchorElement>("a[href]").forEach((link) => {
    const target = resolveSpecNavigationTarget(link.getAttribute("href") || "", spec.path, lookup);
    if (!target) return;
    configureInternalSpecLink(link, target, onNavigate);
  });
}

function decorateInternalDocMentions(
  root: HTMLElement,
  spec: SpecDocument,
  lookup: Map<string, SpecDocument>,
  onNavigate: (target: InternalSpecTarget) => void,
): void {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
    acceptNode: (node) => {
      const parent = node.parentElement;
      if (!parent || parent.closest("a, pre, code, script, style")) {
        return NodeFilter.FILTER_REJECT;
      }
      return internalDocMentionPattern.test(node.textContent || "") ? NodeFilter.FILTER_ACCEPT : NodeFilter.FILTER_REJECT;
    },
  });
  const nodes: Text[] = [];
  while (walker.nextNode()) {
    nodes.push(walker.currentNode as Text);
  }
  nodes.forEach((node) => replaceInternalDocMentions(node, spec, lookup, onNavigate));
}

function replaceInternalDocMentions(
  node: Text,
  spec: SpecDocument,
  lookup: Map<string, SpecDocument>,
  onNavigate: (target: InternalSpecTarget) => void,
): void {
  const text = node.textContent || "";
  const pattern = internalDocMentionPattern;
  const fragment = document.createDocumentFragment();
  let cursor = 0;
  let changed = false;
  for (const match of text.matchAll(pattern)) {
    const raw = match[0];
    const index = match.index || 0;
    const target = resolveSpecNavigationTarget(raw, spec.path, lookup);
    if (!target) continue;
    fragment.append(document.createTextNode(text.slice(cursor, index)));
    fragment.append(createInternalSpecAnchor(raw, target, onNavigate));
    cursor = index + raw.length;
    changed = true;
  }
  if (!changed) return;
  fragment.append(document.createTextNode(text.slice(cursor)));
  node.replaceWith(fragment);
}

const internalDocMentionPattern =
  /@(?:doc|spec)\/[A-Za-z0-9_./-]+(?:#[A-Za-z0-9_-]+)?|(?:\.{1,2}\/|docs\/|specs\/)?[A-Za-z0-9_./-]+\.(?:md|html?)(?:#[A-Za-z0-9_-]+)?/g;

function createInternalSpecAnchor(
  label: string,
  target: InternalSpecTarget,
  onNavigate: (target: InternalSpecTarget) => void,
): HTMLAnchorElement {
  const link = document.createElement("a");
  link.textContent = label;
  configureInternalSpecLink(link, target, onNavigate);
  return link;
}

function configureInternalSpecLink(
  link: HTMLAnchorElement,
  target: InternalSpecTarget,
  onNavigate: (target: InternalSpecTarget) => void,
): void {
  link.href = `/spec/${encodeURIComponent(target.specId)}${target.fragment ? `#${encodeURIComponent(target.fragment)}` : ""}`;
  link.dataset.internalSpecLink = target.specId;
  if (target.fragment) {
    link.dataset.internalSpecFragment = target.fragment;
  }
  link.addEventListener("click", (event) => {
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) return;
    event.preventDefault();
    onNavigate(target);
  });
}

function resolveSpecNavigationTarget(value: string, sourcePath: string, lookup: Map<string, SpecDocument>): InternalSpecTarget | null {
  const parsed = parseInternalSpecTargetValue(value);
  if (!parsed) return null;
  for (const candidate of specPathCandidates(parsed.path, sourcePath)) {
    const spec = lookup.get(candidate);
    if (spec) {
      return { specId: spec.id, fragment: parsed.fragment };
    }
  }
  return null;
}

function parseInternalSpecTargetValue(value: string): { path: string; fragment: string } | null {
  let target = String(value || "").trim();
  if (!target || target.startsWith("#") || isExternalHref(target)) return null;
  if (target.startsWith("@doc/") || target.startsWith("@spec/")) {
    target = target.replace(/^@(doc|spec)\//, "");
  }
  if (target.startsWith("/spec/")) {
    const [routePath, fragment = ""] = target.slice("/spec/".length).split("#", 2);
    return { path: decodeURIComponent(routePath), fragment };
  }
  const hashIndex = target.indexOf("#");
  const fragment = hashIndex >= 0 ? target.slice(hashIndex + 1) : "";
  const path = hashIndex >= 0 ? target.slice(0, hashIndex) : target;
  if (!path || path.includes("://")) return null;
  return { path: decodeURIComponent(path), fragment };
}

function isExternalHref(value: string): boolean {
  return /^[a-z][a-z0-9+.-]*:/i.test(value) || value.startsWith("//");
}

function buildSpecLookup(specs: SpecDocument[]): Map<string, SpecDocument> {
  const lookup = new Map<string, SpecDocument>();
  specs.forEach((spec) => {
    for (const alias of specAliases(spec)) {
      const key = normalizeSpecLookupKey(alias);
      if (key && !lookup.has(key)) {
        lookup.set(key, spec);
      }
    }
  });
  return lookup;
}

function specAliases(spec: SpecDocument): string[] {
  const pathNoExt = spec.path.replace(/\.(?:md|html?)$/i, "");
  const basename = spec.path.split("/").pop() || spec.path;
  const basenameNoExt = basename.replace(/\.(?:md|html?)$/i, "");
  const title = (spec.title || "").trim().toLowerCase();
  return [
    spec.id,
    spec.path,
    `docs/${spec.path}`,
    `specs/${spec.path}`,
    pathNoExt,
    `${pathNoExt}.md`,
    `${pathNoExt}.html`,
    pathNoExt.replace(/-/g, "."),
    pathNoExt.replace(/\./g, "-"),
    basename,
    basenameNoExt,
    `${basenameNoExt}.md`,
    `${basenameNoExt}.html`,
    basenameNoExt.replace(/-/g, "."),
    title,
    title.replace(/\s+/g, "-"),
    title.replace(/\s+/g, "."),
    slugifySpecText(spec.title || ""),
    slugifySpecText((spec.title || "").replace(/\s+/g, ".")),
  ];
}

function specPathCandidates(path: string, sourcePath: string): string[] {
  const candidates = new Set<string>();
  const add = (candidate: string) => {
    const key = normalizeSpecLookupKey(candidate);
    if (!key) return;
    candidates.add(key);
    if (!key.endsWith(".md") && !key.endsWith(".html") && !key.endsWith(".htm") && !key.includes(".")) {
      candidates.add(`${key}.md`);
      candidates.add(`${key}.html`);
      candidates.add(`${key}/_overview.md`);
      candidates.add(`${key}/_overview.html`);
    }
  };
  add(path);
  if (!path.startsWith("/")) {
    add(joinSpecPath(sourcePath.split("/").slice(0, -1).join("/"), path));
  }
  return [...candidates];
}

function normalizeSpecLookupKey(value: string): string {
  let key = String(value || "").trim();
  if (!key) return "";
  key = key.replace(/^@(doc|spec)\//, "");
  key = key.split(/[?#]/, 1)[0] || "";
  key = key.replace(/^\/+/, "");
  key = key.replace(/^\.\//, "");
  key = key.replace(/^docs\//, "");
  key = key.replace(/^specs\//, "");
  key = normalizePathSegments(key);
  return key === "." ? "" : key.toLowerCase();
}

function joinSpecPath(base: string, target: string): string {
  if (!base || target.startsWith("/") || target.startsWith("docs/") || target.startsWith("specs/")) {
    return target;
  }
  return `${base}/${target}`;
}

function normalizePathSegments(path: string): string {
  const segments: string[] = [];
  path.split("/").forEach((segment) => {
    if (!segment || segment === ".") return;
    if (segment === "..") {
      segments.pop();
      return;
    }
    segments.push(segment);
  });
  return segments.join("/");
}

function slugifySpecText(value: string): string {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}
