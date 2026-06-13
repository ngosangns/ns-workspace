<script setup lang="ts">
import { onMounted, onUnmounted, provide, ref } from "vue";
import SearchPanel from "./components/SearchPanel.vue";
import PreviewModal from "./components/PreviewModal.vue";
import Icon from "./components/Icon.vue";
import {
  ProjectKey,
  SpecsKey,
  ThemeKey,
  SelectSpecKey,
  OpenSpecPreviewKey,
  OpenFilePreviewKey,
  ClosePreviewKey,
  ToggleThemeKey,
  type SpecDocument,
  type ProjectSummary,
  type PreviewSource,
  type ThemePreference,
} from "./js/shared-types.js";
import { fetchJSON } from "./js/shared-utils.js";

const project = ref<ProjectSummary | null>(null);
const specs = ref<SpecDocument[]>([]);
const theme = ref<"light" | "dark">("light");
const themePreference = ref<ThemePreference>("system");
const searchQuery = ref("");
const searchKeywordOperator = ref("sum");
const previewSource = ref<PreviewSource | null>(null);
const previewShowRaw = ref(false);
const systemThemeQuery = window.matchMedia?.("(prefers-color-scheme: dark)") || null;

function handlePopState() {
  const next = routeFromLocation();
  searchQuery.value = next.query;
  searchKeywordOperator.value = next.keywordOperator;
}

function getInitialThemePreference(): ThemePreference {
  const stored = localStorage.getItem("spec-preview-theme");
  if (stored === "dark" || stored === "light") return stored;
  return "system";
}

function resolvedSystemTheme(): "light" | "dark" {
  return systemThemeQuery?.matches ? "dark" : "light";
}

function resolveThemePreference(preference: ThemePreference): "light" | "dark" {
  return preference === "system" ? resolvedSystemTheme() : preference;
}

function applyThemePreference(preference: ThemePreference, options: { persist?: boolean } = {}) {
  themePreference.value = preference;
  const resolvedTheme = resolveThemePreference(preference);
  theme.value = resolvedTheme;
  document.documentElement.dataset.theme = resolvedTheme;
  if (!options.persist) return;
  if (preference === "system") {
    localStorage.removeItem("spec-preview-theme");
  } else {
    localStorage.setItem("spec-preview-theme", preference);
  }
}

function toggleTheme() {
  const next = themePreference.value === "system" ? "dark" : themePreference.value === "dark" ? "light" : "system";
  applyThemePreference(next, { persist: true });
}

function handleSystemThemeChange() {
  if (themePreference.value === "system") {
    applyThemePreference("system");
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

function routeFromLocation(): { query: string; keywordOperator: string } {
  const params = new URLSearchParams(window.location.search);
  return {
    query: params.get("q") || "",
    keywordOperator: params.get("keywordOp") === "difference" ? "difference" : "sum",
  };
}

function replaceRouteURL() {
  const params = new URLSearchParams();
  const query = searchQuery.value.trim();
  if (query) params.set("q", query);
  if (searchKeywordOperator.value === "difference") params.set("keywordOp", "difference");
  const next = `${window.location.pathname}${params.toString() ? `?${params.toString()}` : ""}`;
  const current = `${window.location.pathname}${window.location.search}`;
  if (next !== current) {
    window.history.replaceState({}, "", next);
  }
}

function updateSearchQuery(query: string) {
  searchQuery.value = query;
  replaceRouteURL();
}

function updateSearchKeywordOperator(keywordOperator: string) {
  searchKeywordOperator.value = keywordOperator === "difference" ? "difference" : "sum";
  replaceRouteURL();
}

async function selectSpec(id: string) {
  await openSpecPreview(id);
}

async function openSpecPreview(id: string) {
  if (!id) return;
  try {
    const spec = await fetchJSON(`/api/docs/${encodeURIComponent(id)}`);
    previewSource.value = {
      type: "doc",
      raw: spec.raw || "",
      language: spec.language || "markdown",
      path: spec.path || id,
      line: 0,
      spec,
    };
    previewShowRaw.value = false;
  } catch (error) {
    console.error(error);
  }
}

async function openFilePreview(path: string, line: number) {
  if (!path) return;
  try {
    const data = await fetchJSON(`/api/files?${new URLSearchParams({ path }).toString()}`);
    previewSource.value =
      data.type === "folder"
        ? { type: "file", raw: JSON.stringify(data, null, 2), language: "json", path, line: 0 }
        : {
            type: "file",
            raw: data.raw || "",
            language: data.language || "text",
            path: data.path || path,
            line: Number(line || 0),
          };
    previewShowRaw.value = false;
  } catch (error) {
    console.error(error);
  }
}

function closePreview() {
  previewSource.value = null;
  previewShowRaw.value = false;
}

provide(ProjectKey, project);
provide(SpecsKey, specs);
provide(ThemeKey, theme);
provide(SelectSpecKey, selectSpec);
provide(OpenSpecPreviewKey, openSpecPreview);
provide(OpenFilePreviewKey, openFilePreview);
provide(ClosePreviewKey, closePreview);
provide(ToggleThemeKey, toggleTheme);

onMounted(async () => {
  applyThemePreference(getInitialThemePreference());
  const route = routeFromLocation();
  searchQuery.value = route.query;
  searchKeywordOperator.value = route.keywordOperator;
  window.addEventListener("popstate", handlePopState);
  systemThemeQuery?.addEventListener("change", handleSystemThemeChange);

  try {
    const [proj, specList] = await Promise.all([fetchJSON("/api/project"), fetchJSON("/api/docs")]);
    project.value = proj;
    specs.value = specList;
  } catch (error) {
    console.error(error);
  }
});

onUnmounted(() => {
  window.removeEventListener("popstate", handlePopState);
  systemThemeQuery?.removeEventListener("change", handleSystemThemeChange);
});
</script>

<template>
  <main class="min-h-screen bg-c-bg text-c-text">
    <header class="sticky top-0 z-20 border-b border-c-border bg-c-surface/95 px-4 py-4 backdrop-blur">
      <div class="mx-auto flex max-w-7xl flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div class="min-w-0">
          <h1 class="truncate text-lg font-semibold tracking-tight">Graph search</h1>
          <p class="truncate text-xs text-c-text-secondary font-mono">
            {{ project?.projectRoot || project?.name || "Project" }}
          </p>
        </div>
        <button
          class="btn btn-ghost btn-sm"
          type="button"
          :aria-label="themeToggleLabel()"
          :title="themeToggleLabel()"
          @click="toggleTheme"
        >
          <Icon :name="themeToggleIcon()" class="h-4 w-4" />
        </button>
      </div>
    </header>

    <section class="mx-auto max-w-7xl p-4">
      <SearchPanel
        :query="searchQuery"
        :keyword-operator="searchKeywordOperator"
        @update:query="updateSearchQuery"
        @update:keyword-operator="updateSearchKeywordOperator"
        @open-spec-preview="openSpecPreview"
        @open-file-preview="openFilePreview"
      />
    </section>

    <PreviewModal :source="previewSource" :show-raw="previewShowRaw" @close="closePreview" @toggle-raw="previewShowRaw = !previewShowRaw" />
  </main>
</template>
