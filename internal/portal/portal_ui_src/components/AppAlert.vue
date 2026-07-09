<script setup lang="ts">
withDefaults(
  defineProps<{
    kind?: "error" | "success" | "info" | "warning";
    title?: string;
  }>(),
  {
    kind: "error",
    title: "",
  },
);

const kindClass: Record<string, string> = {
  error: "border-negative/35 bg-negative/10",
  success: "border-positive/35 bg-positive/10",
  warning: "border-warning/35 bg-warning/10",
  info: "border-accent-ring bg-accent-soft",
};

const titleClass: Record<string, string> = {
  error: "text-negative",
  success: "text-positive",
  warning: "text-warning",
  info: "text-accent",
};
</script>

<template>
  <div v-if="$slots.default" :class="['mb-4 rounded-md border px-3.5 py-3', kindClass[kind]]" role="alert">
    <div v-if="title" :class="['mb-0.5 text-[13px] font-semibold tracking-tight', titleClass[kind]]">
      {{ title }}
    </div>
    <div class="text-[13px] leading-snug text-fg-secondary">
      <slot />
    </div>
  </div>
</template>
