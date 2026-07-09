import { onUnmounted, ref } from "vue";

/** Short-lived success/info flash with automatic clear. */
export function useFlashMessage(ttlMs = 3200) {
  const message = ref("");
  let timer: ReturnType<typeof setTimeout> | null = null;

  function clear() {
    message.value = "";
    if (timer) {
      clearTimeout(timer);
      timer = null;
    }
  }

  function flash(text: string) {
    clear();
    message.value = text;
    timer = setTimeout(() => {
      message.value = "";
      timer = null;
    }, ttlMs);
  }

  onUnmounted(clear);

  return { message, flash, clear };
}
