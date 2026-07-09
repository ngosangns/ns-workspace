<script setup lang="ts">
export interface SelectOption {
  label: string;
  value: string;
}

defineProps<{
  modelValue: string;
  options: SelectOption[];
  disabled?: boolean;
  id?: string;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", value: string): void;
}>();
</script>

<template>
  <select
    :id="id"
    :value="modelValue"
    :disabled="disabled"
    class="min-w-[160px] flex-1 appearance-none rounded-md border border-border bg-app-muted px-3 py-2 pr-8 text-[13px] font-medium text-fg outline-none transition duration-160 ease-[var(--ease-out-soft)] hover:border-border-strong focus:border-accent disabled:cursor-not-allowed disabled:opacity-50"
    @change="emit('update:modelValue', ($event.target as HTMLSelectElement).value)"
  >
    <option v-for="opt in options" :key="opt.value" :value="opt.value">
      {{ opt.label }}
    </option>
  </select>
</template>
