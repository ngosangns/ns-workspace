<script setup lang="ts">
import { ref, onMounted } from "vue";
import { PhArrowSquareOut } from "@phosphor-icons/vue";
import { api, type Adapter } from "../api";
import AppAlert from "../components/AppAlert.vue";

const adapters = ref<Adapter[]>([]);
const loading = ref(true);
const error = ref("");
const toggling = ref<Record<string, boolean>>({});

function tierClass(tier: string): string {
  switch (tier) {
    case "stable":
      return "status-pill--ok";
    case "manual":
      return "status-pill--warn";
    case "experimental":
      return "status-pill--muted";
    default:
      return "status-pill--muted";
  }
}

async function load() {
  loading.value = true;
  error.value = "";
  try {
    adapters.value = await api.getAdapters();
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    loading.value = false;
  }
}

async function toggleEnabled(adapter: Adapter, next: boolean) {
  toggling.value = { ...toggling.value, [adapter.id]: true };
  error.value = "";
  try {
    const updated = await api.setAdapterEnabled(adapter.id, next);
    adapters.value = adapters.value.map((a) => (a.id === adapter.id ? { ...a, enabled: updated.enabled } : a));
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    toggling.value = { ...toggling.value, [adapter.id]: false };
  }
}

onMounted(load);
</script>

<template>
  <div>
    <header class="page-header fade-in-up">
      <h1 class="page-title">Adapters</h1>
      <p class="page-subtitle">
        {{
          loading
            ? "Loading adapters..."
            : `${adapters.length} providers · ${adapters.filter((a) => a.enabled).length} enabled · ${adapters.filter((a) => !a.enabled).length} disabled. Disable keeps providers listed; they are skipped during sync.`
        }}
      </p>
    </header>

    <AppAlert v-if="error" kind="error">{{ error }}</AppAlert>

    <div v-else-if="loading" class="surface overflow-hidden fade-in-up is-visible" aria-busy="true" aria-label="Loading adapters">
      <div class="space-y-0 divide-y divide-border p-0">
        <div v-for="n in 6" :key="n" class="px-4 py-3">
          <div class="skeleton h-12 rounded-md" />
        </div>
      </div>
    </div>

    <div
      v-else-if="adapters.length === 0"
      class="fade-in-up is-visible rounded-lg border border-dashed border-border-strong bg-surface px-5 py-12 text-center"
    >
      <p class="m-0 mb-1.5 text-[15px] font-semibold text-fg">No adapters configured</p>
      <p class="m-0 text-[13px] text-fg-muted">Provider adapters appear here once presets are available.</p>
    </div>

    <div v-else class="surface overflow-hidden fade-in-up">
      <ul class="m-0 list-none divide-y divide-border p-0">
        <li
          v-for="adapter in adapters"
          :key="adapter.id"
          class="flex flex-wrap items-start gap-x-4 gap-y-2 px-4 py-3 transition duration-160 ease-[var(--ease-out-soft)] hover:bg-elevated"
          :class="adapter.enabled ? '' : 'opacity-60'"
        >
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-[14px] font-semibold tracking-tight text-fg">{{ adapter.name }}</span>
              <span class="font-mono text-[11.5px] text-fg-muted">{{ adapter.id }}</span>
              <span :class="['status-pill', tierClass(adapter.tier)]">{{ adapter.tier }}</span>
            </div>
            <p v-if="adapter.notes" class="m-0 mt-1 text-[13px] leading-normal text-fg-secondary">
              {{ adapter.notes }}
            </p>
            <div
              v-if="(adapter.artifacts && adapter.artifacts.length) || (adapter.docs && adapter.docs.length)"
              class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1.5"
            >
              <div v-if="adapter.artifacts && adapter.artifacts.length" class="flex flex-wrap gap-1.5">
                <span
                  v-for="artifact in adapter.artifacts"
                  :key="artifact"
                  class="rounded-sm border border-border bg-app-muted px-2 py-[2px] font-mono text-[11px] text-fg-secondary"
                >
                  {{ artifact }}
                </span>
              </div>
              <div v-if="adapter.docs && adapter.docs.length" class="flex flex-wrap gap-2.5">
                <a
                  v-for="doc in adapter.docs"
                  :key="doc"
                  :href="doc"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex items-center gap-1 text-xs font-medium text-accent transition duration-160 ease-[var(--ease-out-soft)] hover:text-accent-hover hover:underline hover:underline-offset-2"
                >
                  <PhArrowSquareOut :size="14" weight="bold" />
                  Docs
                </a>
              </div>
            </div>
          </div>
          <div class="flex shrink-0 items-center gap-3 self-center">
            <span :class="['status-pill', adapter.enabled ? 'status-pill--ok' : 'status-pill--muted']">
              {{ adapter.enabled ? "Enabled" : "Disabled" }}
            </span>
            <label class="flex items-center gap-2">
              <span class="text-[11px] font-medium uppercase tracking-wide text-fg-muted">
                {{ adapter.enabled ? "On" : "Off" }}
              </span>
              <input
                type="checkbox"
                class="h-4 w-4 accent-[var(--color-accent)]"
                :checked="adapter.enabled"
                :disabled="toggling[adapter.id]"
                :aria-label="`Enable provider ${adapter.name}`"
                @change="toggleEnabled(adapter, ($event.target as HTMLInputElement).checked)"
              />
            </label>
          </div>
        </li>
      </ul>
    </div>
  </div>
</template>
