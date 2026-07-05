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
    <h2 class="text-h5 q-mb-md">Dashboard</h2>

    <q-banner v-if="error" class="bg-negative text-white q-mb-md" rounded>{{ error }}</q-banner>
    <div v-else-if="loading" class="flex flex-center q-pa-xl">
      <q-spinner color="primary" size="3em" />
    </div>
    <template v-else>
      <div class="row q-col-gutter-md q-mb-md">
        <div class="col-12 col-sm-6 col-md-3">
          <q-card class="bg-secondary" bordered>
            <q-card-section class="text-center">
              <div class="text-h3 text-primary">{{ skills.length }}</div>
              <div class="text-caption text-grey-5">SKILLS</div>
            </q-card-section>
          </q-card>
        </div>
        <div class="col-12 col-sm-6 col-md-3">
          <q-card class="bg-secondary" bordered>
            <q-card-section class="text-center">
              <div class="text-h3 text-primary">{{ Object.keys(mcps?.mcpServers || {}).length }}</div>
              <div class="text-caption text-grey-5">MCP SERVERS</div>
            </q-card-section>
          </q-card>
        </div>
        <div class="col-12 col-sm-6 col-md-3">
          <q-card class="bg-secondary" bordered>
            <q-card-section class="text-center">
              <div class="text-h3 text-primary">{{ registry?.skills.length || 0 }}</div>
              <div class="text-caption text-grey-5">REGISTRY SKILLS</div>
            </q-card-section>
          </q-card>
        </div>
        <div class="col-12 col-sm-6 col-md-3">
          <q-card class="bg-secondary" bordered>
            <q-card-section class="text-center">
              <div class="text-h3 text-primary">{{ adapters.length }}</div>
              <div class="text-caption text-grey-5">ADAPTERS</div>
            </q-card-section>
          </q-card>
        </div>
      </div>

      <q-card class="bg-secondary q-mb-md" bordered>
        <q-card-section>
          <div class="text-h6">Shared Agents Home</div>
          <div class="text-caption text-grey-5">{{ status?.agentsDir }}</div>
        </q-card-section>
        <q-separator />
        <q-markup-table flat class="bg-secondary text-grey-1">
          <thead>
            <tr>
              <th class="text-left">Path</th>
              <th class="text-left">Exists</th>
              <th class="text-left">Type</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in status?.paths" :key="p.path">
              <td>{{ p.path }}</td>
              <td>{{ p.exists ? "Yes" : "No" }}</td>
              <td>{{ p.isDir ? "Directory" : "File" }}</td>
            </tr>
          </tbody>
        </q-markup-table>
      </q-card>

      <SyncPanel @done="load" />
    </template>
  </div>
</template>
