<script setup lang="ts">
import { ref, onMounted } from "vue";
import { PhFolder, PhFile } from "@phosphor-icons/vue";
import { api, type Skill, type MCPServers, type RegistrySkills, type Adapter, type StatusSummary } from "../api";
import AppAlert from "../components/AppAlert.vue";
import SyncPanel from "../components/SyncPanel.vue";

const skills = ref<Skill[]>([]);
const mcps = ref<MCPServers | null>(null);
const registry = ref<RegistrySkills | null>(null);
const adapters = ref<Adapter[]>([]);
const status = ref<StatusSummary | null>(null);
const loading = ref(true);
const error = ref("");

async function load(showLoading = true) {
  if (showLoading) {
    loading.value = true;
  }
  error.value = "";
  try {
    const [s, m, r, a, st] = await Promise.all([api.getSkills(), api.getMCPs(), api.getRegistry(), api.getAdapters(), api.getStatus()]);
    skills.value = s;
    mcps.value = m;
    registry.value = r;
    adapters.value = a;
    status.value = st;
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    if (showLoading) {
      loading.value = false;
    }
  }
}

onMounted(load);
</script>

<template>
  <div>
    <header class="page-header fade-in-up">
      <h1 class="page-title">Dashboard</h1>
      <p class="page-subtitle">Overview of agents, skills, and integrations on this machine.</p>
    </header>

    <AppAlert v-if="error" kind="error">{{ error }}</AppAlert>

    <div v-else-if="loading" class="fade-in-up is-visible" aria-busy="true" aria-label="Loading dashboard">
      <div class="grid grid-cols-1 gap-4 md:grid-cols-[1.2fr_1fr]">
        <div class="skeleton min-h-[220px] md:row-span-2" />
        <div class="skeleton h-[120px]" />
        <div class="skeleton h-[120px]" />
        <div class="skeleton h-[120px]" />
        <div class="skeleton h-[120px]" />
      </div>
      <div class="mt-6 space-y-2">
        <div class="skeleton h-12" />
        <div class="skeleton h-12" />
        <div class="skeleton h-12" />
      </div>
    </div>

    <template v-else>
      <section class="mb-7 fade-in-up" aria-label="Summary metrics">
        <div class="grid grid-cols-1 gap-3 md:grid-cols-[1.35fr_1fr_1fr] md:grid-rows-[auto_auto]">
          <div
            class="surface flex min-h-[220px] flex-col justify-end p-[18px] md:row-span-2"
            style="background: linear-gradient(165deg, var(--color-accent-soft), transparent 55%), var(--color-surface)"
          >
            <div class="mb-2.5 text-[11px] font-semibold uppercase tracking-wider text-fg-muted">Skills</div>
            <div class="mb-2.5 text-[2.75rem] font-semibold leading-none tracking-tighter text-fg tabular-nums">
              {{ skills.length }}
            </div>
            <div class="mb-2.5 text-[13px] font-medium text-fg-secondary">Installed on this host</div>
            <div class="mt-auto flex flex-wrap gap-1.5">
              <span class="status-pill status-pill--accent">{{ skills.filter((s) => s.overridden).length }} overridden</span>
              <span class="status-pill status-pill--muted">{{ skills.filter((s) => !s.enabled).length }} disabled</span>
            </div>
          </div>

          <div class="surface p-[18px] transition duration-160 ease-[var(--ease-out-soft)] hover:border-border-strong hover:bg-elevated">
            <div class="mb-2.5 text-[11px] font-semibold uppercase tracking-wider text-fg-muted">MCP</div>
            <div class="mb-1.5 text-[1.625rem] font-semibold leading-none tracking-tight text-fg tabular-nums">
              {{ Object.keys(mcps?.mcpServers || {}).length }}
            </div>
            <div class="text-[13px] font-medium text-fg-secondary">Servers</div>
          </div>

          <div class="surface p-[18px] transition duration-160 ease-[var(--ease-out-soft)] hover:border-border-strong hover:bg-elevated">
            <div class="mb-2.5 text-[11px] font-semibold uppercase tracking-wider text-fg-muted">Registry</div>
            <div class="mb-1.5 text-[1.625rem] font-semibold leading-none tracking-tight text-fg tabular-nums">
              {{ registry?.skills.length || 0 }}
            </div>
            <div class="text-[13px] font-medium text-fg-secondary">Skills</div>
          </div>

          <div class="surface p-[18px] transition duration-160 ease-[var(--ease-out-soft)] hover:border-border-strong hover:bg-elevated">
            <div class="mb-2.5 text-[11px] font-semibold uppercase tracking-wider text-fg-muted">Adapters</div>
            <div class="mb-1.5 text-[1.625rem] font-semibold leading-none tracking-tight text-fg tabular-nums">
              {{ adapters.length }}
            </div>
            <div class="text-[13px] font-medium text-fg-secondary">Providers</div>
          </div>

          <div
            class="surface p-[18px] transition duration-160 ease-[var(--ease-out-soft)] hover:border-border-strong hover:bg-elevated md:col-span-2"
          >
            <div class="mb-2.5 text-[11px] font-semibold uppercase tracking-wider text-fg-muted">Agents home</div>
            <div
              class="overflow-hidden text-ellipsis whitespace-nowrap font-mono text-[13px] text-fg"
              :title="status?.agentsDir || undefined"
            >
              {{ status?.agentsDir || "not set" }}
            </div>
          </div>
        </div>
      </section>

      <section class="page-section fade-in-up">
        <h2 class="page-section-title">Path status</h2>
        <div class="bezel">
          <div class="bezel-inner">
            <div v-if="!status?.paths?.length" class="px-[18px] py-[18px] text-[13px] text-fg-muted">No path status reported.</div>
            <div
              v-for="p in status?.paths"
              :key="p.path"
              class="flex flex-wrap items-center gap-3 border-b border-border px-4 py-3.5 transition duration-160 ease-[var(--ease-out-soft)] last:border-b-0 hover:bg-hover"
            >
              <component :is="p.isDir ? PhFolder : PhFile" :size="18" class="shrink-0 text-fg-muted" />
              <span
                class="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap font-mono text-[12.5px] text-fg max-md:w-full max-md:whitespace-normal max-md:break-words"
              >
                {{ p.path }}
              </span>
              <span :class="['status-pill', p.exists ? 'status-pill--ok' : 'status-pill--err']">
                {{ p.exists ? "Exists" : "Missing" }}
              </span>
              <span class="w-[70px] shrink-0 text-right text-xs text-fg-muted max-md:ml-auto max-md:w-auto max-md:text-left">
                {{ p.isDir ? "Directory" : "File" }}
              </span>
            </div>
          </div>
        </div>
      </section>

      <section class="page-section fade-in-up">
        <SyncPanel @done="load(false)" />
      </section>
    </template>
  </div>
</template>
