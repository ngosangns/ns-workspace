<script setup lang="ts">
import { ref, watch } from "vue";

interface SearchResult {
  id?: string;
  nodeId?: string;
  title?: string;
  path?: string;
  line?: number;
  specId?: string;
  description?: string;
  excerpt?: string;
  kind?: string;
  score?: number;
  community?: string;
  relation?: string;
  confidence?: string;
  anchor?: boolean;
  anchorId?: string;
  depth?: number;
  matchedBy?: string[];
  neighbors?: SearchNeighbor[];
}

interface SearchNeighbor {
  id?: string;
  label?: string;
  path?: string;
  line?: number;
  relation?: string;
  confidence?: string;
}

interface SearchResponse {
  query?: string;
  mode?: string;
  keywordOperator?: string;
  warnings?: string[];
  stats?: Record<string, number>;
  panels?: Record<string, SearchResult[]>;
}

interface Props {
  query: string;
  keywordOperator: string;
}

const props = defineProps<Props>();
const emit = defineEmits<{
  (e: "openSpecPreview", id: string): void;
  (e: "openFilePreview", path: string, line: number): void;
}>();

const searchQuery = ref("");
const keywordOperator = ref("sum");
const searchLoading = ref(false);
const searchData = ref<SearchResponse | null>(null);
const searchTimer = ref<ReturnType<typeof setTimeout> | null>(null);
const searchController = ref<AbortController | null>(null);

async function fetchJSON(path: string) {
  const res = await fetch(path);
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

async function runSearch() {
  const query = searchQuery.value.trim();
  if (!query) {
    searchData.value = null;
    searchLoading.value = false;
    return;
  }
  if (searchController.value) {
    searchController.value.abort();
  }
  searchLoading.value = true;
  searchController.value = new AbortController();
  const params = new URLSearchParams({ q: query, limit: "8" });
  if (keywordOperator.value !== "sum") {
    params.set("keywordOp", keywordOperator.value);
  }
  try {
    const data = await fetchJSON(`/api/search?${params.toString()}`);
    searchData.value = data;
    searchLoading.value = false;
  } catch (error) {
    if (error.name !== "AbortError") {
      searchLoading.value = false;
      console.error(error);
    }
  }
}

function scheduleSearch() {
  window.clearTimeout(searchTimer.value || undefined);
  searchTimer.value = setTimeout(() => runSearch(), 180);
}

watch(searchQuery, () => {
  scheduleSearch();
});

function renderSearchSummary(): string {
  if (searchLoading.value) {
    return `
      <div class="flex flex-wrap items-center gap-2" aria-live="polite">
        <span class="loading loading-spinner loading-sm text-primary"></span>
        <span class="text-sm text-base-content/70">Searching docs, code, and graphs...</span>
      </div>
    `;
  }
  if (!searchData.value) {
    return '<span class="text-sm text-base-content/60">Search across docs, code, and graph context.</span>';
  }
  const stats = searchData.value.stats || {};
  const total = Object.values(stats).reduce((sum, v) => sum + Number(v || 0), 0);
  const warnings = searchData.value.warnings || [];
  return `
    <div class="flex flex-wrap items-center gap-2">
      <span class="badge badge-primary badge-sm">${escapeHTML(searchData.value.mode || "hybrid")}</span>
      <span class="badge badge-ghost badge-sm">${keywordOperator.value === "difference" ? "keyword difference" : "keyword sum"}</span>
      <span class="badge badge-ghost badge-sm">${total} results</span>
      ${warnings
        .slice(0, 2)
        .map((w) => `<span class="badge badge-warning badge-sm max-w-full truncate">${escapeHTML(w)}</span>`)
        .join("")}
    </div>
  `;
}

function panelResults(panelName: string): SearchResult[] {
  return searchData.value?.panels?.[panelName] || [];
}

function escapeHTML(str: string): string {
  return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#039;");
}
</script>

<template>
  <div class="search-shell border-base-300 bg-base-100 border">
    <div class="search-toolbar border-base-300 border-b">
      <label class="input input-bordered flex min-h-11 flex-1 items-center gap-2">
        <i data-lucide="search" class="text-base-content/50 h-4 w-4"></i>
        <input v-model="searchQuery" class="grow" placeholder="Search docs, code, graph nodes; separate keywords with commas" />
      </label>
      <select
        v-model="keywordOperator"
        class="select select-bordered min-h-11"
        aria-label="Keyword result operator"
        title="Keyword result operator"
      >
        <option value="sum">Sum keywords</option>
        <option value="difference">Difference keywords</option>
      </select>
    </div>
    <div id="searchSummary" class="search-summary border-base-300 border-b" v-html="renderSearchSummary()"></div>
    <div class="search-grid">
      <section class="search-panel" data-search-panel="docsSemantic">
        <div class="search-panel-heading">
          <h2>Docs Semantic</h2>
          <span class="badge badge-ghost badge-sm">{{ panelResults("docsSemantic").length }}</span>
        </div>
        <div class="search-results">
          <article v-for="result in panelResults('docsSemantic')" :key="result.id || result.nodeId" class="search-result">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <h3>{{ result.title || result.id || "Untitled" }}</h3>
                <p v-if="result.path" class="search-path">{{ result.path }}</p>
              </div>
              <span class="badge badge-outline badge-sm shrink-0">{{ Math.round((result.score || 0) * 100) }}%</span>
            </div>
            <p v-if="result.description || result.excerpt" class="search-excerpt">{{ result.description || result.excerpt }}</p>
            <div class="mt-2 flex flex-wrap gap-2">
              <button v-if="result.specId" class="btn btn-primary btn-xs" type="button" @click="emit('openSpecPreview', result.specId)">
                <i data-lucide="file-text" class="h-3.5 w-3.5"></i>Preview doc
              </button>
              <button
                v-if="result.path && result.specId"
                class="btn btn-outline btn-xs"
                type="button"
                @click="emit('openFilePreview', result.path, result.line || 0)"
              >
                <i data-lucide="file-code" class="h-3.5 w-3.5"></i>Preview file
              </button>
            </div>
          </article>
          <div v-if="panelResults('docsSemantic').length === 0 && !searchLoading" class="search-empty">No document semantic matches.</div>
        </div>
      </section>

      <section class="search-panel" data-search-panel="docsGraph">
        <div class="search-panel-heading">
          <h2>Docs Graph</h2>
          <span class="badge badge-ghost badge-sm">{{ panelResults("docsGraph").length }}</span>
        </div>
        <div class="search-results">
          <div v-if="panelResults('docsGraph').length === 0 && !searchLoading" class="search-empty">No document graph matches.</div>
        </div>
      </section>

      <section class="search-panel" data-search-panel="codeSemantic">
        <div class="search-panel-heading">
          <h2>Code Semantic</h2>
          <span class="badge badge-ghost badge-sm">{{ panelResults("codeSemantic").length }}</span>
        </div>
        <div class="search-results">
          <article v-for="result in panelResults('codeSemantic')" :key="result.id || result.nodeId" class="search-result">
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <h3>{{ result.title || result.id || "Untitled" }}</h3>
                <p v-if="result.path" class="search-path">{{ result.path }}</p>
              </div>
              <span class="badge badge-outline badge-sm shrink-0">{{ Math.round((result.score || 0) * 100) }}%</span>
            </div>
            <p v-if="result.description || result.excerpt" class="search-excerpt">{{ result.description || result.excerpt }}</p>
            <div class="mt-2 flex flex-wrap gap-2">
              <button
                v-if="result.path"
                class="btn btn-outline btn-xs"
                type="button"
                @click="emit('openFilePreview', result.path, result.line || 0)"
              >
                <i data-lucide="file-code" class="h-3.5 w-3.5"></i>Preview file
              </button>
            </div>
          </article>
          <div v-if="panelResults('codeSemantic').length === 0 && !searchLoading" class="search-empty">No code semantic matches.</div>
        </div>
      </section>

      <section class="search-panel" data-search-panel="codeGraph">
        <div class="search-panel-heading">
          <h2>Code Graph</h2>
          <span class="badge badge-ghost badge-sm">{{ panelResults("codeGraph").length }}</span>
        </div>
        <div class="search-results">
          <div v-if="panelResults('codeGraph').length === 0 && !searchLoading" class="search-empty">No code graph matches.</div>
        </div>
      </section>
    </div>
  </div>
</template>
