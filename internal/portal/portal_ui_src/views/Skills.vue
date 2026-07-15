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
const toggling = ref<Record<string, boolean>>({});

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

async function toggleEnabled(skill: Skill, next: boolean) {
  toggling.value = { ...toggling.value, [skill.id]: true };
  error.value = "";
  try {
    const updated = await api.setSkillEnabled(skill.id, next);
    skills.value = skills.value.map((s) =>
      s.id === skill.id ? { ...s, enabled: updated.enabled, description: updated.description ?? s.description } : s,
    );
  } catch (e: any) {
    error.value = e.message || String(e);
  } finally {
    toggling.value = { ...toggling.value, [skill.id]: false };
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
    selected.value = { ...skill, ...s };
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

function description(skill: Skill): string {
  const d = skill.description?.trim();
  if (d) return d;
  return "No description in skill frontmatter.";
}

onMounted(load);
</script>

<template>
  <div>
    <header class="page-header fade-in-up">
      <h1 class="page-title">Skills</h1>
      <p class="page-subtitle">
        {{
          loading
            ? "Loading skills..."
            : `${skills.length} skills · ${skills.filter((s) => s.enabled).length} enabled · ${skills.filter((s) => !s.enabled).length} disabled. Disable keeps skills listed; they are skipped during sync.`
        }}
      </p>
    </header>

    <AppAlert v-if="error" kind="error">{{ error }}</AppAlert>

    <div v-else-if="loading" class="surface overflow-hidden fade-in-up is-visible" aria-busy="true" aria-label="Loading skills">
      <div class="space-y-0 divide-y divide-border p-0">
        <div v-for="n in 6" :key="n" class="px-4 py-3">
          <div class="skeleton h-14 rounded-md" />
        </div>
      </div>
    </div>

    <div
      v-else-if="skills.length === 0"
      class="fade-in-up is-visible rounded-lg border border-dashed border-border-strong bg-surface px-5 py-12 text-center"
    >
      <p class="m-0 mb-1.5 text-[15px] font-semibold text-fg">No skills found</p>
      <p class="m-0 text-[13px] text-fg-muted">Sync or install skills to see them listed here.</p>
    </div>

    <div v-else class="surface overflow-hidden fade-in-up">
      <ul class="m-0 list-none divide-y divide-border p-0">
        <li
          v-for="skill in skills"
          :key="skill.id"
          class="flex flex-wrap items-start gap-x-4 gap-y-2 px-4 py-3 transition duration-160 ease-[var(--ease-out-soft)] hover:bg-elevated"
          :class="skill.enabled ? '' : 'opacity-60'"
        >
          <button type="button" class="min-w-0 flex-1 text-left" @click="open(skill)">
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-[14px] font-semibold tracking-tight text-fg">{{ skill.name }}</span>
              <span v-if="skill.name !== skill.id" class="font-mono text-[11.5px] text-fg-muted">{{ skill.id }}</span>
            </div>
            <p class="m-0 mt-1 line-clamp-2 text-[13px] leading-normal text-fg-secondary">
              {{ description(skill) }}
            </p>
            <div class="mt-1.5 font-mono text-[11.5px] text-fg-muted">{{ skill.source }}</div>
          </button>
          <div class="flex shrink-0 flex-wrap items-center gap-2 self-center">
            <span :class="['status-pill', skill.enabled ? 'status-pill--ok' : 'status-pill--muted']">
              {{ skill.enabled ? "Enabled" : "Disabled" }}
            </span>
            <span :class="['status-pill', skill.overridden ? 'status-pill--accent' : 'status-pill--muted']">
              {{ skill.overridden ? "Custom" : "Default" }}
            </span>
            <UiButton v-if="skill.overridden" size="sm" variant="danger" @click="reset(skill.id)">
              <PhArrowCounterClockwise :size="14" weight="bold" />
              Reset
            </UiButton>
            <label class="flex items-center gap-2" @click.stop>
              <span class="text-[11px] font-medium uppercase tracking-wide text-fg-muted">
                {{ skill.enabled ? "On" : "Off" }}
              </span>
              <input
                type="checkbox"
                class="h-4 w-4 accent-[var(--color-accent)]"
                :checked="skill.enabled"
                :disabled="toggling[skill.id]"
                :aria-label="`Enable skill ${skill.name}`"
                @change="toggleEnabled(skill, ($event.target as HTMLInputElement).checked)"
              />
            </label>
          </div>
        </li>
      </ul>
    </div>

    <UiDialog :open="dialog" :title="selected?.name || 'Skill'" :subtitle="selected?.description || selected?.source" @close="closeDialog">
      <AppAlert v-if="dialogError" kind="error">{{ dialogError }}</AppAlert>
      <div v-else-if="dialogLoading" aria-busy="true">
        <div class="skeleton h-[420px]" />
      </div>
      <CodeEditor v-else v-model="content" lang="markdown" readonly />
    </UiDialog>
  </div>
</template>
