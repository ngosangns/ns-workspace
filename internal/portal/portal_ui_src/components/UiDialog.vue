<script setup lang="ts">
import { onMounted, onUnmounted, watch } from "vue";
import { PhX } from "@phosphor-icons/vue";
import UiButton from "./UiButton.vue";

const props = defineProps<{
  open: boolean;
  title?: string;
  subtitle?: string;
}>();

const emit = defineEmits<{
  (e: "close"): void;
}>();

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape" && props.open) {
    emit("close");
  }
}

watch(
  () => props.open,
  (open) => {
    document.body.style.overflow = open ? "hidden" : "";
  },
);

onMounted(() => window.addEventListener("keydown", onKeydown));
onUnmounted(() => {
  window.removeEventListener("keydown", onKeydown);
  document.body.style.overflow = "";
});
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-[2100] flex items-stretch justify-center bg-app/80 p-0 backdrop-blur-sm sm:items-center sm:p-6"
      role="dialog"
      aria-modal="true"
      :aria-label="title || 'Dialog'"
      @click.self="emit('close')"
    >
      <div
        class="flex max-h-full w-full max-w-[960px] flex-col overflow-hidden border border-border bg-surface shadow-[var(--shadow-panel)] sm:max-h-[90dvh] sm:rounded-xl"
      >
        <header class="flex items-start gap-3 border-b border-border px-4 py-3 sm:px-5">
          <div class="min-w-0 flex-1">
            <h2 v-if="title" class="m-0 text-lg font-semibold tracking-tight text-fg">{{ title }}</h2>
            <p v-if="subtitle" class="mt-0.5 font-mono text-xs text-fg-muted">{{ subtitle }}</p>
          </div>
          <UiButton size="icon" variant="ghost" aria-label="Close" @click="emit('close')">
            <PhX :size="18" weight="bold" />
          </UiButton>
        </header>
        <div class="min-h-0 flex-1 overflow-auto p-4 sm:p-5">
          <slot />
        </div>
      </div>
    </div>
  </Teleport>
</template>
