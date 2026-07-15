import { createSignal, onCleanup } from "solid-js";

/** Short-lived success/info flash with automatic clear. */
export function useFlashMessage(ttlMs = 3200) {
  const [message, setMessage] = createSignal("");
  let timer: ReturnType<typeof setTimeout> | null = null;

  function clear() {
    setMessage("");
    if (timer) {
      clearTimeout(timer);
      timer = null;
    }
  }

  function flash(text: string) {
    clear();
    setMessage(text);
    timer = setTimeout(() => {
      setMessage("");
      timer = null;
    }, ttlMs);
  }

  onCleanup(clear);

  return { message, flash, clear };
}
