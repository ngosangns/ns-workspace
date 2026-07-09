<script setup lang="ts">
import { ref, onMounted } from "vue";
import { PhArrowCounterClockwise } from "@phosphor-icons/vue";
import { api, type Skill } from "../api";
import AppAlert from "../components/AppAlert.vue";
import CodeEditor from "../components/CodeEditor.vue";
import UiButton from "../components/UiButton.vue";
import UiDialog from "../components/UiDialog.vue";

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
    <header class="page-header fade-in-up">
      <h1 class="page-title">Skills</h1>
      <p class="page-subtitle">{{ loading ? "Loading skills..." : `${skills.length} skills available across providers.` }}</p>
    </header>

    <AppAlert v-if="error" kind="error">{{ error }}</AppAlert>

    <div v-else-if="loading" class="fade-in-up is-visible" aria-busy="true" aria-label="Loading skills">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <div v-for="n in 6" :key="n" class="skeleton h-[168px]" />
      </div>
    </div>

    <div
      v-else-if="skills.length === 0"
      class="fade-in-up is-visible rounded-lg border border-dashed border-border-strong bg-surface px-5 py-12 text-center"
    >
      <p class="m-0 mb-1.5 text-[15px] font-semibold text-fg">No skills found</p>
      <p class="m-0 text-[13px] text-fg-muted">Sync or install skills to see them listed here.</p>
    </div>

    <div v-else class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
      <button
        v-for="(skill, index) in skills"
        :key="skill.id"
        type="button"
        class="surface flex min-h-[168px] w-full flex-col p-4 text-left transition duration-160 ease-[var(--ease-out-soft)] hover:-translate-y-px hover:border-border-strong hover:bg-elevated hover:shadow-[var(--shadow-soft)] active:scale-[0.995] fade-in-up"
        :style="{ transitionDelay: `${Math.min(index, 8) * 30}ms` }"
        @click="open(skill)"
      >
        <div class="mb-2.5 flex items-start justify-between gap-3">
          <div class="text-[15px] font-semibold leading-snug tracking-tight text-fg">{{ skill.name }}</div>
          <span :class="['status-pill shrink-0', skill.overridden ? 'status-pill--accent' : 'status-pill--muted']">
            {{ skill.overridden ? "Overridden" : "Embedded" }}
          </span>
        </div>
        <div class="line-clamp-3 flex-1 text-[13px] leading-normal text-fg-secondary">{{ preview(skill) }}</div>
        <div class="mt-3.5 flex items-center justify-between gap-2 border-t border-border pt-3">
          <span class="min-w-0 overflow-hidden text-ellipsis whitespace-nowrap font-mono text-[11.5px] text-fg-muted">
            {{ skill.source }}
          </span>
          <UiButton v-if="skill.overridden" size="sm" variant="danger" class="shrink-0" @click.stop="reset(skill.id)">
            <PhArrowCounterClockwise :size="14" weight="bold" />
            Reset
          </UiButton>
        </div>
      </button>
    </div>

    <UiDialog :open="dialog" :title="selected?.name || 'Skill'" :subtitle="selected?.source" @close="closeDialog">
      <AppAlert v-if="dialogError" kind="error">{{ dialogError }}</AppAlert>
      <div v-else-if="dialogLoading" aria-busy="true">
        <div class="skeleton h-[420px]" />
      </div>
      <CodeEditor v-else v-model="content" lang="markdown" readonly />
    </UiDialog>
  </div>
</template>
