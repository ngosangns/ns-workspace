<script setup lang="ts">
import { ref, onMounted } from "vue";
import { api, type Skill } from "../api";
import CodeEditor from "../components/CodeEditor.vue";

const skills = ref<Skill[]>([]);
const loading = ref(true);
const error = ref("");

const dialog = ref(false);
const selected = ref<Skill | null>(null);
const content = ref("");
const dialogLoading = ref(false);
const dialogError = ref("");

async function load() {
  loading.value = true;
  error.value = "";
  try {
    skills.value = await api.getSkills();
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    loading.value = false;
  }
}

async function reset(id: string) {
  try {
    await api.resetSkill(id);
    await load();
  } catch (e: any) {
    error.value = e.message || String(e);
  }
}

async function open(skill: Skill) {
  selected.value = skill;
  content.value = "";
  dialogError.value = "";
  dialog.value = true;
  dialogLoading.value = true;
  try {
    const s = await api.getSkill(skill.id);
    content.value = s.content || "";
  } catch (e: any) {
    dialogError.value = e.message || String(e);
  } finally {
    dialogLoading.value = false;
  }
}

function closeDialog() {
  dialog.value = false;
  selected.value = null;
  content.value = "";
  dialogError.value = "";
}

function preview(skill: Skill) {
  return skill.content?.slice(0, 140).replace(/\s+/g, " ").trim() || "No preview available.";
}

onMounted(load);
</script>

<template>
  <div>
    <header class="skills-header fade-in-up">
      <div>
        <h1 class="skills-title">Skills</h1>
        <p class="skills-subtitle">{{ skills.length }} skills available across providers.</p>
      </div>
    </header>

    <q-banner v-if="error" class="bg-negative text-white q-mb-lg rounded-borders" rounded>{{ error }}</q-banner>
    <div v-else-if="loading" class="flex flex-center q-pa-xl">
      <q-spinner color="primary" size="3em" />
    </div>
    <div v-else class="row q-col-gutter-md">
      <div v-for="skill in skills" :key="skill.id" class="col-12 col-sm-6 col-md-4 fade-in-up">
        <div class="skill-card" @click="open(skill)">
          <div class="skill-card-header">
            <div class="skill-card-name">{{ skill.name }}</div>
            <q-badge :color="skill.overridden ? 'primary' : 'grey-7'" text-color="dark" class="skill-card-badge" rounded>
              {{ skill.overridden ? "Overridden" : "Embedded" }}
            </q-badge>
          </div>
          <div class="skill-card-preview">{{ preview(skill) }}</div>
          <div class="skill-card-footer">
            <span class="skill-card-source">{{ skill.source }}</span>
            <q-btn
              v-if="skill.overridden"
              flat
              dense
              color="negative"
              icon="sym_o_restore"
              label="Reset"
              class="skill-card-reset"
              @click.stop="reset(skill.id)"
            />
          </div>
        </div>
      </div>
    </div>

    <q-dialog v-model="dialog" maximized @hide="closeDialog">
      <q-card class="skill-dialog">
        <q-card-section class="row items-center q-pb-none">
          <div>
            <div class="text-h6">{{ selected?.name || "Skill" }}</div>
            <div class="text-caption text-secondary">{{ selected?.source }}</div>
          </div>
          <q-space />
          <q-btn icon="sym_o_close" flat round dense @click="closeDialog" />
        </q-card-section>

        <q-card-section>
          <q-banner v-if="dialogError" class="bg-negative text-white q-mb-md rounded-borders" rounded>{{ dialogError }}</q-banner>
          <div v-else-if="dialogLoading" class="flex flex-center q-pa-xl">
            <q-spinner color="primary" size="3em" />
          </div>
          <CodeEditor v-else v-model="content" lang="markdown" readonly />
        </q-card-section>
      </q-card>
    </q-dialog>
  </div>
</template>

<style scoped>
.skills-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 24px;
}

.skills-title {
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -0.02em;
  margin: 0 0 6px;
  color: var(--color-text);
}

.skills-subtitle {
  font-size: 15px;
  color: var(--color-text-secondary);
  margin: 0;
}

.skill-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: var(--radius-lg);
  padding: 18px;
  height: 100%;
  display: flex;
  flex-direction: column;
  cursor: pointer;
  transition:
    border-color var(--transition-fast),
    transform var(--transition-fast),
    box-shadow var(--transition-fast);
}

.skill-card:hover {
  border-color: var(--color-border-strong);
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
}

.skill-card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.skill-card-name {
  font-size: 16px;
  font-weight: 600;
  color: var(--color-text);
  line-height: 1.3;
}

.skill-card-badge {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  padding: 3px 8px;
  flex-shrink: 0;
}

.skill-card-preview {
  font-size: 13px;
  line-height: 1.5;
  color: var(--color-text-secondary);
  flex: 1 1 auto;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.skill-card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid var(--color-border);
}

.skill-card-source {
  font-size: 12px;
  color: var(--color-text-muted);
  font-family: var(--font-mono);
}

.skill-card-reset {
  font-size: 12px;
}

.skill-dialog {
  background: var(--color-surface);
  color: var(--color-text);
  width: 100%;
  max-width: 900px;
}
</style>
