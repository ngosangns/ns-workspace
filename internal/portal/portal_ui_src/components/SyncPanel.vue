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
  <div class="card">
    <h3>Sync</h3>
    <div class="toolbar">
      <button class="btn" :disabled="running" @click="run('status')">Status</button>
      <button class="btn" :disabled="running" @click="run('doctor')">Doctor</button>
      <button class="btn" :disabled="running" @click="run('init')">Init</button>
      <button class="btn" :disabled="running" @click="run('update', true)">Dry Run Update</button>
      <button class="btn primary" :disabled="running" @click="run('update')">Update</button>
      <button class="btn" :disabled="running" @click="run('registry')">Install Registry</button>
    </div>
    <p v-if="error" class="empty" style="color: var(--danger)">{{ error }}</p>
    <div v-if="logs.length === 0 && running" class="empty">Starting...</div>
    <div v-else class="logs">
      <p v-for="(line, i) in logs" :key="i" class="log-line">{{ line }}</p>
    </div>
  </div>
</template>
