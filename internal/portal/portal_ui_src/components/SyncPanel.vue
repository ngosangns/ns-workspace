<script setup lang="ts">
import { ref } from "vue";
import { api, type SyncJob } from "../api";

const emit = defineEmits<{ (e: "done"): void }>();

const logs = ref<string[]>([]);
const running = ref(false);
const error = ref("");
const currentJob = ref<SyncJob | null>(null);

async function run(command: string, dryRun = false) {
  if (running.value) return;
  running.value = true;
  error.value = "";
  logs.value = [];
  currentJob.value = null;

  try {
    const job = await api.startSync(command, dryRun);
    currentJob.value = job;
    stream(job.id);
  } catch (e: any) {
    running.value = false;
    error.value = e.message || String(e);
  }
}

function stream(jobId: string) {
  const es = api.streamSync(jobId);
  es.onmessage = (event) => {
    if (event.data) {
      logs.value.push(event.data);
    }
  };
  es.addEventListener("end", () => {
    es.close();
    running.value = false;
    emit("done");
  });
  es.onerror = () => {
    es.close();
    running.value = false;
  };
}
</script>

<template>
  <div class="sync-panel">
    <h2 class="sync-title">Sync</h2>

    <div class="sync-toolbar">
      <div class="sync-group">
        <span class="sync-group-label">Inspect</span>
        <q-btn :disable="running" flat class="sync-btn" label="Status" @click="run('status')" />
        <q-btn :disable="running" flat class="sync-btn" label="Doctor" @click="run('doctor')" />
      </div>
      <div class="sync-divider" />
      <div class="sync-group">
        <span class="sync-group-label">Modify</span>
        <q-btn :disable="running" flat class="sync-btn" label="Init" @click="run('init')" />
        <q-btn :disable="running" color="primary" class="sync-btn sync-btn--primary" label="Update" @click="run('update')" />
        <q-btn :disable="running" flat class="sync-btn" label="Dry Run" @click="run('update', true)" />
      </div>
      <div class="sync-divider" />
      <div class="sync-group">
        <span class="sync-group-label">Registry</span>
        <q-btn :disable="running" flat class="sync-btn" label="Install" @click="run('registry')" />
      </div>
    </div>

    <q-banner v-if="error" class="bg-negative text-white q-mb-md rounded-borders" rounded>{{ error }}</q-banner>

    <div v-if="logs.length > 0 || running" class="sync-terminal">
      <div class="sync-terminal-header">
        <div class="sync-terminal-dots">
          <span class="sync-dot sync-dot--red" />
          <span class="sync-dot sync-dot--yellow" />
          <span class="sync-dot sync-dot--green" />
        </div>
        <span class="sync-terminal-title">
          <q-spinner v-if="running" color="primary" size="14px" class="q-mr-sm" />
          {{ currentJob?.command || "sync" }} — {{ running ? "running" : "done" }}
        </span>
      </div>
      <q-scroll-area class="sync-scroll">
        <div class="sync-logs">
          <div v-for="(line, i) in logs" :key="i" class="sync-log-line">{{ line }}</div>
          <div v-if="running && logs.length === 0" class="sync-log-line sync-log-line--muted">Starting...</div>
        </div>
      </q-scroll-area>
    </div>
  </div>
</template>

<style scoped>
.sync-panel {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 20px;
}

.sync-title {
  font-size: 18px;
  font-weight: 600;
  letter-spacing: -0.01em;
  margin: 0 0 14px;
  color: var(--color-text);
}

.sync-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 16px;
  margin-bottom: 16px;
}

.sync-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.sync-group-label {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: var(--color-text-muted);
  margin-right: 4px;
}

.sync-divider {
  width: 1px;
  height: 24px;
  background: var(--color-border);
}

.sync-btn {
  color: var(--color-text-secondary);
  border-radius: var(--radius-md);
  padding: 6px 14px;
  font-weight: 600;
}

.sync-btn:hover {
  color: var(--color-text);
  background: rgba(255, 255, 255, 0.05);
}

.sync-btn--primary {
  color: var(--color-bg);
  background: var(--color-accent);
}

.sync-btn--primary:hover {
  background: var(--color-accent-hover);
}

.sync-terminal {
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  overflow: hidden;
}

.sync-terminal-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid var(--color-border);
}

.sync-terminal-dots {
  display: flex;
  gap: 6px;
}

.sync-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.sync-dot--red {
  background: #f87171;
}

.sync-dot--yellow {
  background: #fbbf24;
}

.sync-dot--green {
  background: #34d399;
}

.sync-terminal-title {
  font-size: 12px;
  font-family: var(--font-mono);
  color: var(--color-text-secondary);
}

.sync-scroll {
  height: 300px;
}

.sync-logs {
  padding: 14px;
}

.sync-log-line {
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.6;
  color: var(--color-text-secondary);
  white-space: pre-wrap;
  word-break: break-word;
}

.sync-log-line--muted {
  color: var(--color-text-muted);
}
</style>
