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

async function load() {
  loading.value = true;
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
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <div>
    <h2 class="page-title">Dashboard</h2>
    <p v-if="loading" class="empty">Loading...</p>
    <p v-else-if="error" class="empty" style="color: var(--danger)">{{ error }}</p>
    <template v-else>
      <div class="grid">
        <div class="card metric">
          <div class="value">{{ skills.length }}</div>
          <div class="label">Skills</div>
        </div>
        <div class="card metric">
          <div class="value">{{ Object.keys(mcps?.mcpServers || {}).length }}</div>
          <div class="label">MCP Servers</div>
        </div>
        <div class="card metric">
          <div class="value">{{ registry?.skills.length || 0 }}</div>
          <div class="label">Registry Skills</div>
        </div>
        <div class="card metric">
          <div class="value">{{ adapters.length }}</div>
          <div class="label">Adapters</div>
        </div>
      </div>

      <div class="card">
        <h3>Shared Agents Home</h3>
        <p class="meta">{{ status?.agentsDir }}</p>
        <table class="table">
          <thead>
            <tr>
              <th>Path</th>
              <th>Exists</th>
              <th>Type</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in status?.paths" :key="p.path">
              <td>{{ p.path }}</td>
              <td>{{ p.exists ? "Yes" : "No" }}</td>
              <td>{{ p.isDir ? "Directory" : "File" }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <SyncPanel @done="load" />
    </template>
  </div>
</template>
