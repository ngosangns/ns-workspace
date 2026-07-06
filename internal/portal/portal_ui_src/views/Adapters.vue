<script setup lang="ts">
import { ref, onMounted } from "vue";
import { api, type Adapter } from "../api";

const adapters = ref<Adapter[]>([]);
const loading = ref(true);
const error = ref("");

function tierClass(tier: string): string {
  switch (tier) {
    case "stable":
      return "adapter-tier--stable";
    case "manual":
      return "adapter-tier--manual";
    case "experimental":
      return "adapter-tier--experimental";
    default:
      return "adapter-tier--experimental";
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
    <header class="adapters-header fade-in-up">
      <div>
        <h1 class="adapters-title">Adapters</h1>
        <p class="adapters-subtitle">{{ adapters.length }} providers configured for sync.</p>
      </div>
    </header>

    <q-banner v-if="error" class="bg-negative text-white q-mb-lg rounded-borders" rounded>{{ error }}</q-banner>
    <div v-else-if="loading" class="flex flex-center q-pa-xl">
      <q-spinner color="primary" size="3em" />
    </div>
    <div v-else class="row q-col-gutter-md">
      <div v-for="adapter in adapters" :key="adapter.id" class="col-12 col-sm-6 col-md-4 fade-in-up">
        <div class="adapter-card">
          <div class="adapter-card-header">
            <div class="adapter-card-name">{{ adapter.name }}</div>
            <q-badge :class="['adapter-tier', tierClass(adapter.tier)]" rounded>
              {{ adapter.tier }}
            </q-badge>
          </div>
          <div v-if="adapter.artifacts && adapter.artifacts.length" class="adapter-card-artifacts">
            <span v-for="artifact in adapter.artifacts" :key="artifact" class="adapter-artifact">{{ artifact }}</span>
          </div>
          <div v-if="adapter.notes" class="adapter-card-notes">{{ adapter.notes }}</div>
          <div v-if="adapter.docs && adapter.docs.length" class="adapter-card-docs">
            <a
              v-for="doc in adapter.docs"
              :key="doc"
              :href="doc"
              target="_blank"
              rel="noopener noreferrer"
              class="adapter-doc-link"
              @click.stop
            >
              <q-icon name="sym_o_open_in_new" size="14px" />
              Docs
            </a>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.adapters-header {
  margin-bottom: 24px;
}

.adapters-title {
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -0.02em;
  margin: 0 0 6px;
  color: var(--color-text);
}

.adapters-subtitle {
  font-size: 15px;
  color: var(--color-text-secondary);
  margin: 0;
}

.adapter-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 18px;
  height: 100%;
  transition:
    border-color var(--transition-fast),
    transform var(--transition-fast);
}

.adapter-card:hover {
  border-color: var(--color-border-strong);
  transform: translateY(-1px);
}

.adapter-card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.adapter-card-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text);
  line-height: 1.3;
}

.adapter-tier {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  padding: 3px 8px;
  flex-shrink: 0;
}

.adapter-tier--stable {
  background: rgba(52, 211, 153, 0.15);
  color: var(--color-positive);
}

.adapter-tier--manual {
  background: rgba(251, 191, 36, 0.15);
  color: var(--color-warning);
}

.adapter-tier--experimental {
  background: rgba(156, 163, 175, 0.15);
  color: var(--color-text-secondary);
}

.adapter-card-artifacts {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 12px;
}

.adapter-artifact {
  font-size: 11px;
  font-family: var(--font-mono);
  color: var(--color-text-secondary);
  background: rgba(255, 255, 255, 0.05);
  padding: 3px 8px;
  border-radius: 999px;
}

.adapter-card-notes {
  font-size: 13px;
  line-height: 1.5;
  color: var(--color-text-secondary);
  margin-bottom: 12px;
}

.adapter-card-docs {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.adapter-doc-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--color-accent);
  text-decoration: none;
  transition: color var(--transition-fast);
}

.adapter-doc-link:hover {
  color: var(--color-accent-hover);
  text-decoration: underline;
}
</style>
