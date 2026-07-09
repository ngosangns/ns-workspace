<script setup lang="ts">
import { ref, onMounted } from "vue";
import { api, type Skill, type MCPServers, type RegistrySkills, type Adapter, type StatusSummary } from "../api";
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
    <header class="dash-header fade-in-up">
      <h1 class="dash-title">Dashboard</h1>
      <p class="dash-subtitle">Overview of your agents, skills, and integrations.</p>
    </header>

    <q-banner v-if="error" class="bg-negative text-white q-mb-lg rounded-borders" rounded>{{ error }}</q-banner>
    <div v-else-if="loading" class="flex flex-center q-pa-xl">
      <q-spinner color="primary" size="3em" />
    </div>
    <template v-else>
      <section class="dash-bento fade-in-up">
        <div class="dash-bento-grid">
          <div class="dash-card dash-card--featured">
            <div class="dash-card-icon dash-card-icon--primary">
              <q-icon name="sym_o_psychology" size="28px" />
            </div>
            <div class="dash-card-value">{{ skills.length }}</div>
            <div class="dash-card-label">Skills installed</div>
            <div class="dash-card-meta">{{ skills.filter((s) => s.overridden).length }} overridden</div>
          </div>

          <div class="dash-card">
            <div class="dash-card-icon dash-card-icon--info">
              <q-icon name="sym_o_dns" size="22px" />
            </div>
            <div class="dash-card-value dash-card-value--small">{{ Object.keys(mcps?.mcpServers || {}).length }}</div>
            <div class="dash-card-label">MCP servers</div>
          </div>

          <div class="dash-card">
            <div class="dash-card-icon dash-card-icon--accent">
              <q-icon name="sym_o_apps" size="22px" />
            </div>
            <div class="dash-card-value dash-card-value--small">{{ registry?.skills.length || 0 }}</div>
            <div class="dash-card-label">Registry skills</div>
          </div>

          <div class="dash-card">
            <div class="dash-card-icon dash-card-icon--positive">
              <q-icon name="sym_o_extension" size="22px" />
            </div>
            <div class="dash-card-value dash-card-value--small">{{ adapters.length }}</div>
            <div class="dash-card-label">Adapters</div>
          </div>

          <div class="dash-card">
            <div class="dash-card-icon dash-card-icon--warning">
              <q-icon name="sym_o_folder" size="22px" />
            </div>
            <div class="dash-card-value dash-card-value--small ellipsis">{{ status?.agentsDir || "—" }}</div>
            <div class="dash-card-label">Agents home</div>
          </div>
        </div>
      </section>

      <section class="dash-section fade-in-up">
        <h2 class="dash-section-title">Path status</h2>
        <div class="dash-list">
          <div v-for="p in status?.paths" :key="p.path" class="dash-list-row">
            <q-icon :name="p.isDir ? 'sym_o_folder' : 'sym_o_description'" class="dash-list-icon" size="20px" />
            <span class="dash-list-path">{{ p.path }}</span>
            <q-space />
            <q-badge :color="p.exists ? 'positive' : 'negative'" text-color="white" class="dash-list-badge" rounded>
              {{ p.exists ? "Exists" : "Missing" }}
            </q-badge>
            <span class="dash-list-type">{{ p.isDir ? "Directory" : "File" }}</span>
          </div>
        </div>
      </section>

      <section class="dash-section fade-in-up">
        <SyncPanel @done="load(false)" />
      </section>
    </template>
  </div>
</template>

<style scoped>
.dash-header {
  margin-bottom: 28px;
}

.dash-title {
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -0.02em;
  margin: 0 0 6px;
  color: var(--color-text);
}

.dash-subtitle {
  font-size: 15px;
  color: var(--color-text-secondary);
  margin: 0;
}

.dash-bento {
  margin-bottom: 28px;
}

.dash-bento-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 16px;
}

@media (min-width: 768px) {
  .dash-bento-grid {
    grid-template-columns: 1fr 1fr;
    grid-template-rows: 1fr 1fr;
  }

  .dash-card--featured {
    grid-row: span 2;
  }
}

.dash-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 20px;
  display: flex;
  flex-direction: column;
  transition:
    border-color var(--transition-fast),
    transform var(--transition-fast);
}

.dash-card:hover {
  border-color: var(--color-border-strong);
  transform: translateY(-1px);
}

.dash-card--featured {
  justify-content: center;
  min-height: 240px;
}

.dash-card-icon {
  width: 44px;
  height: 44px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 16px;
}

.dash-card-icon--primary {
  background: rgba(45, 212, 191, 0.12);
  color: var(--color-accent);
}

.dash-card-icon--info {
  background: rgba(56, 189, 248, 0.12);
  color: var(--color-info);
}

.dash-card-icon--accent {
  background: rgba(45, 212, 191, 0.1);
  color: var(--color-accent);
}

.dash-card-icon--positive {
  background: rgba(52, 211, 153, 0.12);
  color: var(--color-positive);
}

.dash-card-icon--warning {
  background: rgba(251, 191, 36, 0.12);
  color: var(--color-warning);
}

.dash-card-value {
  font-size: 42px;
  font-weight: 700;
  letter-spacing: -0.03em;
  line-height: 1;
  color: var(--color-text);
  margin-bottom: 8px;
}

.dash-card-value--small {
  font-size: 24px;
}

.dash-card-label {
  font-size: 13px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--color-text-secondary);
  margin-bottom: 4px;
}

.dash-card-meta {
  font-size: 13px;
  color: var(--color-text-muted);
}

.dash-section {
  margin-bottom: 28px;
}

.dash-section-title {
  font-size: 18px;
  font-weight: 600;
  letter-spacing: -0.01em;
  margin: 0 0 14px;
  color: var(--color-text);
}

.dash-list {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.dash-list-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 18px;
  border-bottom: 1px solid var(--color-border);
  transition: background var(--transition-fast);
  flex-wrap: wrap;
}

.dash-list-row:last-child {
  border-bottom: none;
}

.dash-list-row:hover {
  background: rgba(255, 255, 255, 0.02);
}

.dash-list-icon {
  color: var(--color-text-muted);
}

.dash-list-path {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--color-text);
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1 1 auto;
}

@media (max-width: 767px) {
  .dash-list-path {
    width: 100%;
    white-space: normal;
    word-break: break-word;
  }

  .dash-list-type {
    text-align: left;
    width: auto;
    margin-left: auto;
  }
}

.dash-list-badge {
  font-size: 11px;
  font-weight: 600;
  padding: 3px 10px;
}

.dash-list-type {
  font-size: 12px;
  color: var(--color-text-muted);
  width: 70px;
  text-align: right;
}
</style>
