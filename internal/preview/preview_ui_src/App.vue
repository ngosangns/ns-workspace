<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted, provide } from "vue";
import Sidebar from "./components/Sidebar.vue";
import DocViewer from "./components/DocViewer.vue";
import GraphViewer from "./components/GraphViewer.vue";
import SearchPanel from "./components/SearchPanel.vue";
import PreviewModal from "./components/PreviewModal.vue";
import Icon from "./components/Icon.vue";

interface ProjectSummary {
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

interface SpecDocument {
  id: string;
  title: string;
  path: string;
  raw?: string;
  language?: string;
  status?: string;
  compliance?: string;
}

interface GraphData {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

interface GraphNode {
  id: string;
  label?: string;
  type?: string;
  path?: string;
  specId?: string;
  category?: string;
  status?: string;
  [key: string]: any;
}

interface GraphEdge {
  from: string;
  to: string;
  type?: string;
  label?: string;
  [key: string]: any;
}

interface FolderListing {
  folders: Array<{ path: string; name: string; count: number }>;
  docs: SpecDocument[];
}

type ThemePreference = "system" | "dark" | "light";

const project = ref<ProjectSummary | null>(null);
const specs = ref<SpecDocument[]>([]);
const graph = ref<GraphData | null>(null);
const currentSpec = ref<SpecDocument | null>(null);
const selectedId = ref("");
const selectedFolderPath = ref("");
const routeSpecId = ref("");
const routeFolderPath = ref("");
const tab = ref("spec");
const theme = ref<"light" | "dark">("light");
const themePreference = ref<ThemePreference>("system");
const previewSource = ref<{ type: "doc" | "file"; raw: string; language: string; path: string; line: number; spec?: SpecDocument } | null>(
  null,
);
const previewShowRaw = ref(false);
const graphQuery = ref("");
const searchQuery = ref("");
const searchKeywordOperator = ref("sum");
const systemThemeQuery = window.matchMedia?.("(prefers-color-scheme: dark)") || null;

const selectedFolderListing = computed<FolderListing>(() => specFolderListing(selectedFolderPath.value));

function getInitialThemePreference(): ThemePreference {
  const stored = localStorage.getItem("spec-preview-theme");
  if (stored === "dark" || stored === "light") return stored;
  return "system";
}

async function fetchJSON(path: string) {
  const res = await fetch(path);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

function resolvedSystemTheme(): "light" | "dark" {
  return systemThemeQuery?.matches ? "dark" : "light";
}

function resolveThemePreference(preference: ThemePreference): "light" | "dark" {
  return preference === "system" ? resolvedSystemTheme() : preference;
}

function applyThemePreference(preference: ThemePreference, options: { persist?: boolean; rerender?: boolean } = {}) {
  themePreference.value = preference;
  const resolvedTheme = resolveThemePreference(preference);
  theme.value = resolvedTheme;
  document.documentElement.dataset.theme = resolvedTheme;
  if (options.persist) {
    if (preference === "system") {
      localStorage.removeItem("spec-preview-theme");
    } else {
      localStorage.setItem("spec-preview-theme", preference);
    }
  }
}

function toggleTheme() {
  const next = themePreference.value === "system" ? "dark" : themePreference.value === "dark" ? "light" : "system";
  applyThemePreference(next, { persist: true, rerender: true });
}

function handleSystemThemeChange() {
  if (themePreference.value === "system") {
    applyThemePreference("system", { rerender: true });
  }
}

function themeToggleIcon(): "monitor" | "moon" | "sun" {
  if (themePreference.value === "system") return "monitor";
  return themePreference.value === "dark" ? "moon" : "sun";
}

function themeToggleLabel(): string {
  if (themePreference.value === "system") return "Theme: system";
  return themePreference.value === "dark" ? "Theme: dark" : "Theme: light";
}

function routeFromLocation(): { tab?: string; spec?: string; fragment?: string; searchQuery: string; searchKeywordOperator: string } {
  const path = window.location.pathname;
  const search = window.location.search;
  const hash = window.location.hash;
  const params = new URLSearchParams(search);
  const routeQuery = {
    searchQuery: params.get("q") || "",
    searchKeywordOperator: params.get("keywordOp") === "difference" ? "difference" : "sum",
  };

  if (path === "/graph") {
    return { tab: "graph", spec: undefined, fragment: hash.slice(1) || undefined, ...routeQuery };
  }
  if (path === "/search") {
    return { tab: "search", spec: undefined, fragment: hash.slice(1) || undefined, ...routeQuery };
  }

  const tab = params.get("tab") || undefined;
  const match = path.match(/^\/spec\/(.*)$/);
  if (!match) return { tab, spec: undefined, fragment: hash.slice(1) || undefined, ...routeQuery };
  const spec = decodeURIComponent(match[1]);
  const parts = spec.split("#");
  return { tab, spec: parts[0], fragment: parts[1] || undefined, ...routeQuery };
}

function validSpecId(id: string): string {
  if (!id) return "";
  return specs.value.some((spec) => spec.id === id) ? id : "";
}

function validSpecFolderPath(path: string): string {
  const normalized = normalizeSpecFolderPath(path);
  if (!normalized) return "";
  if (normalized === "docs") return specs.value.length ? normalized : "";
  return specFolderPaths().has(normalized) ? normalized : "";
}

function normalizeSpecFolderPath(path: string): string {
  let normalized = decodeURIComponent(String(path || ""))
    .replace(/\\/g, "/")
    .replace(/^\/+|\/+$/g, "");
  if (!normalized) return "";
  if (normalized.toLowerCase() === "docs") return "docs";
  normalized = normalized.replace(/^docs\/+/i, "").replace(/^specs\/+/i, "specs/");
  return normalized.replace(/\/{2,}/g, "/");
}

function specFolderPaths(): Set<string> {
  const paths = new Set<string>();
  specs.value.forEach((spec) => {
    const parts = spec.path.split("/");
    for (let index = 1; index < parts.length; index++) {
      paths.add(parts.slice(0, index).join("/"));
    }
  });
  return paths;
}

function defaultSpecId(): string {
  return specs.value[0]?.id || "";
}

function folderDisplayName(path: string): string {
  return path.split("/").filter(Boolean).pop() || "Docs";
}

function specFolderListing(folderPath: string): FolderListing {
  const prefix = folderPath === "docs" ? "" : `${folderPath}/`;
  const folders = new Map<string, { path: string; name: string; count: number }>();
  const docs: SpecDocument[] = [];
  specs.value.forEach((spec) => {
    if (!spec.path.startsWith(prefix)) return;
    const rest = spec.path.slice(prefix.length);
    if (!rest || (rest === spec.path && folderPath !== "docs")) return;
    const parts = rest.split("/");
    if (parts.length === 1) {
      docs.push(spec);
      return;
    }
    const name = parts[0];
    const path = folderPath === "docs" ? name : `${folderPath}/${name}`;
    const folder = folders.get(path) || { path, name, count: 0 };
    folder.count += 1;
    folders.set(path, folder);
  });
  return {
    folders: [...folders.values()].sort((a, b) => a.name.localeCompare(b.name)),
    docs: docs.sort((a, b) => a.path.localeCompare(b.path)),
  };
}

function switchTab(tabName: string, options: { updateURL?: boolean } = {}) {
  tab.value = tabName;
  if (tabName === "spec" && !selectedId.value && !selectedFolderPath.value) {
    const defaultId = defaultSpecId();
    if (defaultId) {
      selectedId.value = defaultId;
    }
  }
  if (options.updateURL !== false) {
    updateRouteURL(tabName);
  }
}

function updateRouteURL(tabName: string) {
  const query = buildRouteQuery(tabName);
  if (tabName === "graph") {
    window.history.pushState({}, "", `/graph${query}`);
    return;
  }
  if (tabName === "search") {
    window.history.pushState({}, "", `/search${query}`);
    return;
  }

  const params = new URLSearchParams();
  if (tabName && tabName !== "spec") {
    params.set("tab", tabName);
  }
  const spec = selectedId.value || selectedFolderPath.value;
  const path = spec ? `/spec/${encodeURIComponent(spec)}` : "/spec";
  const queryString = params.toString();
  const url = queryString ? `${path}?${queryString}` : path;
  window.history.pushState({}, "", url);
}

function buildRouteQuery(tabName: string): string {
  const params = new URLSearchParams();
  if (tabName === "graph") {
    const query = graphQuery.value.trim();
    if (query) params.set("q", query);
  }
  if (tabName === "search") {
    const query = searchQuery.value.trim();
    if (query) params.set("q", query);
    if (searchKeywordOperator.value === "difference") {
      params.set("keywordOp", "difference");
    }
  }
  const query = params.toString();
  return query ? `?${query}` : "";
}

function replaceFocusedRouteURL(tabName: string) {
  if (tab.value !== tabName) return;
  const path = tabName === "graph" || tabName === "search" ? `/${tabName}` : window.location.pathname;
  const next = `${path}${buildRouteQuery(tabName)}`;
  const current = `${window.location.pathname}${window.location.search}`;
  if (next !== current) {
    window.history.replaceState({}, "", next);
  }
}

function updateSearchQuery(query: string) {
  searchQuery.value = query;
  replaceFocusedRouteURL("search");
}

function updateGraphQuery(query: string) {
  graphQuery.value = query;
  replaceFocusedRouteURL("graph");
}

function updateSearchKeywordOperator(keywordOperator: string) {
  searchKeywordOperator.value = keywordOperator === "difference" ? "difference" : "sum";
  replaceFocusedRouteURL("search");
}

function setPageChromeForTab(tabName: string) {
  if (tabName === "spec") {
    document.querySelectorAll(".panel").forEach((p) => p.classList.remove("active"));
    document.querySelector("#specTab")?.classList.add("active");
  } else if (tabName === "graph") {
    document.querySelectorAll(".panel").forEach((p) => p.classList.remove("active"));
    document.querySelector("#graphTab")?.classList.add("active");
  } else if (tabName === "search") {
    document.querySelectorAll(".panel").forEach((p) => p.classList.remove("active"));
    document.querySelector("#searchTab")?.classList.add("active");
  }
}

function syncRouteSpecFromURL(route: { tab?: string; spec?: string; fragment?: string }) {
  // Keep resolved route values separate from refs so URL sync updates Vue state.
  const resolvedSpecId = validSpecId(route.spec || "");
  const resolvedFolderPath = validSpecFolderPath(route.spec || "");
  routeSpecId.value = resolvedSpecId;
  routeFolderPath.value = resolvedFolderPath;
}

async function handlePopState() {
  const route = routeFromLocation();
  if (route.tab === "graph") {
    graphQuery.value = route.searchQuery;
  } else {
    searchQuery.value = route.searchQuery;
  }
  searchKeywordOperator.value = route.searchKeywordOperator;
  syncRouteSpecFromURL(route);

  if (route.tab === "graph") {
    switchTab("graph", { updateURL: false });
    return;
  }
  if (route.tab === "search") {
    switchTab("search", { updateURL: false });
    return;
  }

  // Sync tab from URL without triggering another URL update.
  if (route.tab === "spec") {
    switchTab("spec", { updateURL: false });
  } else {
    switchTab("spec", { updateURL: false });
  }

  // Sync spec or folder selection from URL without triggering another URL update.
  const resolvedSpecId = validSpecId(route.spec || "");
  const resolvedFolderPath = validSpecFolderPath(route.spec || "");
  if (resolvedFolderPath) {
    await selectSpecFolder(resolvedFolderPath, false, { updateURL: false });
  } else if (resolvedSpecId) {
    await selectSpec(resolvedSpecId, false, { updateURL: false });
  }
}

async function selectSpec(id: string, showSpecTab = true, options: { updateURL?: boolean } = {}) {
  const updateURL = options.updateURL !== false;
  const spec = await fetchJSON(`/api/docs/${encodeURIComponent(id)}`);
  selectedId.value = id;
  selectedFolderPath.value = "";
  currentSpec.value = spec;
  if (tab.value === "spec" || showSpecTab) {
    setPageChromeForTab("spec");
  }
  if (showSpecTab) {
    switchTab("spec", { updateURL });
  } else if (updateURL && tab.value === "spec") {
    updateRouteURL("spec");
  }
}

async function selectSpecFolder(path: string, showSpecTab = true, options: { updateURL?: boolean } = {}) {
  const folderPath = validSpecFolderPath(path);
  if (!folderPath) return;
  selectedId.value = "";
  selectedFolderPath.value = folderPath;
  currentSpec.value = null;
  if (tab.value === "spec" || showSpecTab) {
    setPageChromeForTab("spec");
  }
  if (showSpecTab) {
    switchTab("spec", { updateURL: options.updateURL });
  } else if (options.updateURL !== false && tab.value === "spec") {
    updateRouteURL("spec");
  }
}

async function selectSpecTreeItem(idOrPath: string) {
  if (validSpecFolderPath(idOrPath)) {
    await selectSpecFolder(idOrPath, true);
    return;
  }
  await selectSpec(idOrPath, true);
}

function openSpecPreview(id: string, options: { updateURL?: boolean } = {}) {
  if (!id) return;
  fetchJSON(`/api/docs/${encodeURIComponent(id)}`)
    .then((spec) => {
      previewSource.value = {
        type: "doc",
        raw: spec.raw || "",
        language: spec.language || "markdown",
        path: spec.path || id,
        line: 0,
        spec,
      };
      previewShowRaw.value = false;
    })
    .catch(console.error);
}

function openFilePreview(path: string, line: number, options: { updateURL?: boolean } = {}) {
  if (!path) return;
  fetchJSON(`/api/files?${new URLSearchParams({ path }).toString()}`)
    .then((data) => {
      if (data.type === "folder") {
        previewSource.value = { type: "file", raw: JSON.stringify(data, null, 2), language: "json", path, line: 0 };
      } else {
        previewSource.value = {
          type: "file",
          raw: data.raw || "",
          language: data.language || "text",
          path: data.path || path,
          line: Number(line || 0),
        };
      }
      previewShowRaw.value = false;
    })
    .catch(console.error);
}

function closePreview() {
  previewSource.value = null;
  previewShowRaw.value = false;
}

provide("project", project);
provide("specs", specs);
provide("graph", graph);
provide("currentSpec", currentSpec);
provide("selectedId", selectedId);
provide("selectedFolderPath", selectedFolderPath);
provide("tab", tab);
provide("theme", theme);
provide("searchQuery", searchQuery);
provide("searchKeywordOperator", searchKeywordOperator);
provide("selectSpec", selectSpec);
provide("openSpecPreview", openSpecPreview);
provide("openFilePreview", openFilePreview);
provide("closePreview", closePreview);
provide("toggleTheme", toggleTheme);

onMounted(async () => {
  applyThemePreference(getInitialThemePreference());
  const [proj, specList, graphData] = await Promise.all([fetchJSON("/api/project"), fetchJSON("/api/docs"), fetchJSON("/api/graph")]);
  project.value = proj;
  specs.value = specList;
  graph.value = graphData;
  const route = routeFromLocation();
  if (route.tab === "graph") {
    graphQuery.value = route.searchQuery;
  } else {
    searchQuery.value = route.searchQuery;
  }
  searchKeywordOperator.value = route.searchKeywordOperator;
  syncRouteSpecFromURL(route);

  if (route.tab !== "graph") {
    selectedId.value = validSpecId(route.spec || "") || (validSpecFolderPath(route.spec || "") ? "" : defaultSpecId());
    selectedFolderPath.value = validSpecFolderPath(route.spec || "");
    if (selectedFolderPath.value) {
      await selectSpecFolder(selectedFolderPath.value, false, { updateURL: false });
    } else if (selectedId.value) {
      await selectSpec(selectedId.value, false, { updateURL: false });
    }
  }

  if (!route.tab && !route.spec && selectedId.value) {
    updateRouteURL("spec");
  }
  switchTab(route.tab || "spec", { updateURL: false });
  window.addEventListener("popstate", handlePopState);
  systemThemeQuery?.addEventListener("change", handleSystemThemeChange);
});

onUnmounted(() => {
  window.removeEventListener("popstate", handlePopState);
  systemThemeQuery?.removeEventListener("change", handleSystemThemeChange);
});
</script>

<template>
  <div class="min-h-screen lg:pl-[22rem]">
    <Sidebar
      :project="project"
      :specs="specs"
      :selected-id="selectedId"
      :selected-folder-path="selectedFolderPath"
      @select-spec="selectSpecTreeItem"
    />

    <main class="min-w-0">
      <header
        class="bg-base-200/90 border-base-300 sticky top-0 z-10 flex flex-col gap-4 border-b px-5 py-4 backdrop-blur md:flex-row md:items-center md:justify-between"
      >
        <div class="min-w-0">
          <h1 id="pageTitle" class="truncate text-2xl font-bold tracking-normal">
            {{ selectedFolderPath ? folderDisplayName(selectedFolderPath) : currentSpec?.title || "Doc" }}
          </h1>
          <p id="pagePath" class="text-base-content/60 truncate text-sm">
            {{ selectedFolderPath || currentSpec?.path || "" }}
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <div role="tablist" class="tabs tabs-boxed w-fit" aria-label="Preview sections">
            <button
              class="tab"
              :class="{ 'tab-active': tab === 'graph' }"
              data-tab="graph"
              type="button"
              aria-label="Graph"
              title="Graph"
              @click="switchTab('graph')"
            >
              <Icon name="git-fork" class="h-4 w-4" />
            </button>
            <button
              class="tab"
              :class="{ 'tab-active': tab === 'search' }"
              data-tab="search"
              type="button"
              aria-label="Search"
              title="Search"
              @click="switchTab('search')"
            >
              <Icon name="search" class="h-4 w-4" />
            </button>
            <button
              id="themeToggle"
              class="tab"
              :data-theme-option="themePreference"
              type="button"
              :aria-label="themeToggleLabel()"
              :title="themeToggleLabel()"
              @click="toggleTheme"
            >
              <Icon :name="themeToggleIcon()" class="h-4 w-4" />
            </button>
          </div>
        </div>
      </header>

      <section id="graphTab" class="panel p-5" :class="{ active: tab === 'graph' }">
        <GraphViewer
          :graph="graph"
          :theme="theme"
          :active="tab === 'graph'"
          :query="graphQuery"
          @update:query="updateGraphQuery"
          @open-spec-preview="openSpecPreview"
        />
      </section>

      <section id="searchTab" class="panel p-5" :class="{ active: tab === 'search' }">
        <SearchPanel
          :query="searchQuery"
          :keyword-operator="searchKeywordOperator"
          @update:query="updateSearchQuery"
          @update:keyword-operator="updateSearchKeywordOperator"
          @open-spec-preview="openSpecPreview"
          @open-file-preview="openFilePreview"
        />
      </section>

      <section id="specTab" class="panel p-5" :class="{ active: tab === 'spec' }">
        <article
          v-if="selectedFolderPath"
          class="card border-base-300 bg-base-100 mx-auto max-w-5xl border p-6"
          :data-source-path="selectedFolderPath"
        >
          <div class="mb-5">
            <p class="text-base-content/60 text-sm font-medium">{{ selectedFolderPath }}</p>
            <h1 class="mt-1 text-3xl font-semibold tracking-normal">{{ folderDisplayName(selectedFolderPath) }}</h1>
          </div>
          <div>
            <button
              v-for="folder in selectedFolderListing.folders"
              :key="folder.path"
              class="btn btn-ghost h-auto min-h-12 w-full justify-start gap-3 rounded border border-base-300 px-3 py-2 text-left"
              type="button"
              @click="selectSpecFolder(folder.path, true)"
            >
              <Icon name="folder" class="h-4 w-4 shrink-0 text-base-content/60" />
              <span class="min-w-0">
                <span class="block truncate font-medium">{{ folder.name }}</span>
                <span class="text-base-content/55 block text-xs">{{ folder.count }} docs</span>
              </span>
            </button>
            <button
              v-for="doc in selectedFolderListing.docs"
              :key="doc.id"
              class="btn btn-ghost h-auto min-h-12 w-full justify-start gap-3 rounded border border-base-300 px-3 py-2 text-left"
              type="button"
              @click="selectSpec(doc.id, true)"
            >
              <Icon name="file-text" class="h-4 w-4 shrink-0 text-base-content/60" />
              <span class="min-w-0">
                <span class="block truncate font-medium">{{ doc.title || doc.path }}</span>
                <span class="text-base-content/55 block truncate text-xs">{{ doc.path }}</span>
              </span>
            </button>
          </div>
          <div
            v-if="!selectedFolderListing.folders.length && !selectedFolderListing.docs.length"
            class="bg-base-200/50 border-base-300 text-base-content/65 rounded border p-4 text-sm"
          >
            No docs in this folder.
          </div>
        </article>
        <DocViewer v-else :spec="currentSpec" :theme="theme" />
      </section>
    </main>

    <PreviewModal :source="previewSource" :show-raw="previewShowRaw" @close="closePreview" @toggle-raw="previewShowRaw = !previewShowRaw" />
  </div>
</template>
