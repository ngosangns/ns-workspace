<script setup lang="ts">
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    variant?: "primary" | "secondary" | "ghost" | "danger" | "warning";
    size?: "sm" | "md" | "icon";
    type?: "button" | "submit" | "reset";
    disabled?: boolean;
    loading?: boolean;
  }>(),
  {
    variant: "secondary",
    size: "md",
    type: "button",
    disabled: false,
    loading: false,
  },
);

const classes = computed(() => {
  const base =
    "inline-flex items-center justify-center gap-1.5 font-semibold tracking-tight transition duration-160 ease-[var(--ease-out-soft)] disabled:cursor-not-allowed disabled:opacity-50 active:scale-[0.98]";

  const sizes = {
    sm: "min-h-8 rounded-md px-3 text-xs",
    md: "min-h-[34px] rounded-md px-3.5 text-[13px]",
    icon: "h-9 w-9 rounded-full p-0",
  } as const;

  const variants = {
    primary: "bg-accent text-ink hover:bg-accent-hover",
    secondary: "bg-transparent text-fg-secondary hover:bg-hover hover:text-fg",
    ghost: "bg-transparent text-fg-secondary hover:bg-hover hover:text-fg",
    danger: "bg-transparent text-negative hover:bg-negative/10",
    warning: "border border-warning/40 bg-transparent text-warning hover:bg-warning/10",
  } as const;

  return [base, sizes[props.size], variants[props.variant]].join(" ");
});
</script>

<template>
  <button :type="type" :class="classes" :disabled="disabled || loading" :aria-busy="loading || undefined">
    <span
      v-if="loading"
      class="inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent"
      aria-hidden="true"
    />
    <slot />
  </button>
</template>
