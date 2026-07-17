import { createSignal, onCleanup } from "solid-js";
import { errMessage } from "./errors";

/** Page-level error + short-lived success flash. */
export function usePageFeedback(ttlMs = 3200) {
  const [error, setError] = createSignal("");
  const [success, setSuccess] = createSignal("");
  let timer: ReturnType<typeof setTimeout> | null = null;

  function clearSuccess() {
    setSuccess("");
    if (timer) {
      clearTimeout(timer);
      timer = null;
    }
  }

  function clearError() {
    setError("");
  }

  function clear() {
    clearError();
    clearSuccess();
  }

  function flash(text: string) {
    clearSuccess();
    setSuccess(text);
    timer = setTimeout(() => {
      setSuccess("");
      timer = null;
    }, ttlMs);
  }

  function fail(e: unknown) {
    clearSuccess();
    setError(errMessage(e));
  }

  onCleanup(clearSuccess);

  return {
    error,
    success,
    setError,
    flash,
    fail,
    clear,
    clearError,
    clearSuccess,
  };
}
