<script setup lang="ts">
import { ref, onMounted } from "vue";
import { api, type Adapter } from "../api";

const adapters = ref<Adapter[]>([]);
const loading = ref(true);
const error = ref("");

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

onMounted(load);
</script>

<template>
  <div>
    <h2 class="page-title">Adapters</h2>
    <p v-if="loading" class="empty">Loading...</p>
    <p v-else-if="error" class="empty" style="color: var(--danger)">{{ error }}</p>
    <div v-else class="list">
      <div v-for="adapter in adapters" :key="adapter.id" class="list-item">
        <div>
          <div class="title">{{ adapter.name }}</div>
          <div class="meta">
            <span :class="['badge', adapter.tier]">{{ adapter.tier }}</span>
            <span v-if="adapter.artifacts">{{ adapter.artifacts.join(", ") }}</span>
          </div>
          <div v-if="adapter.notes" class="meta">{{ adapter.notes }}</div>
        </div>
      </div>
    </div>
  </div>
</template>
