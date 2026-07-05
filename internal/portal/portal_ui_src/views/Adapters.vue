<script setup lang="ts">
import { ref, onMounted } from "vue";
import { api, type Adapter } from "../api";

const adapters = ref<Adapter[]>([]);
const loading = ref(true);
const error = ref("");

function tierColor(tier: string): string {
  switch (tier) {
    case "stable":
      return "positive";
    case "manual":
      return "warning";
    case "experimental":
      return "grey-7";
    default:
      return "grey-7";
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

onMounted(load);
</script>

<template>
  <div>
    <h2 class="text-h5 q-mb-md">Adapters</h2>

    <q-banner v-if="error" class="bg-negative text-white q-mb-md" rounded>{{ error }}</q-banner>
    <div v-else-if="loading" class="flex flex-center q-pa-xl">
      <q-spinner color="primary" size="3em" />
    </div>
    <q-list v-else bordered separator class="bg-secondary rounded-borders">
      <q-item v-for="adapter in adapters" :key="adapter.id" class="q-py-md">
        <q-item-section>
          <q-item-label class="text-weight-medium">{{ adapter.name }}</q-item-label>
          <q-item-label caption>
            <q-chip :color="tierColor(adapter.tier)" text-color="white" size="sm">{{ adapter.tier }}</q-chip>
            <span v-if="adapter.artifacts" class="q-ml-sm text-grey-5">{{ adapter.artifacts.join(", ") }}</span>
          </q-item-label>
          <q-item-label v-if="adapter.notes" caption class="q-mt-xs">{{ adapter.notes }}</q-item-label>
        </q-item-section>
      </q-item>
    </q-list>
  </div>
</template>
