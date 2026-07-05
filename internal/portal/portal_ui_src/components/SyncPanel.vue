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
  <q-card class="bg-secondary" bordered>
    <q-card-section>
      <div class="text-h6">Sync</div>
    </q-card-section>
    <q-separator />
    <q-card-section>
      <div class="row q-gutter-sm q-mb-md">
        <q-btn :disable="running" color="secondary" label="Status" @click="run('status')" />
        <q-btn :disable="running" color="secondary" label="Doctor" @click="run('doctor')" />
        <q-btn :disable="running" color="secondary" label="Init" @click="run('init')" />
        <q-btn :disable="running" color="info" label="Dry Run Update" @click="run('update', true)" />
        <q-btn :disable="running" color="primary" label="Update" @click="run('update')" />
        <q-btn :disable="running" color="secondary" label="Install Registry" @click="run('registry')" />
      </div>

      <q-banner v-if="error" class="bg-negative text-white q-mb-md" rounded>{{ error }}</q-banner>

      <q-scroll-area v-if="logs.length > 0 || running" class="bg-dark rounded-borders" style="height: 300px">
        <div class="q-pa-sm text-mono text-caption">
          <div v-for="(line, i) in logs" :key="i" class="log-line">{{ line }}</div>
          <div v-if="running && logs.length === 0" class="text-grey-5">Starting...</div>
        </div>
      </q-scroll-area>
    </q-card-section>
  </q-card>
</template>

<style scoped>
.log-line {
  white-space: pre-wrap;
  word-break: break-word;
  padding: 1px 0;
}
</style>
