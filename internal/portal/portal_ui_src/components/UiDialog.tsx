import type { JSX, ParentProps } from "solid-js";
import { Show, createEffect, onCleanup } from "solid-js";
import { Portal } from "solid-js/web";
import { PhX } from "./Icons";
import UiButton from "./UiButton";

export default function UiDialog(
  props: ParentProps<{
    open: boolean;
    title?: string;
    subtitle?: string;
    onClose: () => void;
  }>,
): JSX.Element {
  createEffect(() => {
    document.body.style.overflow = props.open ? "hidden" : "";
  });

  const onKeydown = (e: KeyboardEvent) => {
    if (e.key === "Escape" && props.open) props.onClose();
  };

  window.addEventListener("keydown", onKeydown);
  onCleanup(() => {
    window.removeEventListener("keydown", onKeydown);
    document.body.style.overflow = "";
  });

  return (
    <Portal>
      <Show when={props.open}>
        <div
          class="fixed inset-0 z-[2100] flex items-stretch justify-center bg-app/80 p-0 backdrop-blur-sm sm:items-center sm:p-6"
          role="dialog"
          aria-modal="true"
          aria-label={props.title || "Dialog"}
          onClick={(e) => {
            if (e.target === e.currentTarget) props.onClose();
          }}
        >
          <div class="flex max-h-full w-full max-w-[960px] flex-col overflow-hidden border border-border bg-surface shadow-[var(--shadow-panel)] sm:max-h-[90dvh] sm:rounded-xl">
            <header class="flex items-start gap-3 border-b border-border px-4 py-3 sm:px-5">
              <div class="min-w-0 flex-1">
                <Show when={props.title}>
                  <h2 class="m-0 text-lg font-semibold tracking-tight text-fg">{props.title}</h2>
                </Show>
                <Show when={props.subtitle}>
                  <p class="mt-0.5 font-mono text-xs text-fg-muted">{props.subtitle}</p>
                </Show>
              </div>
              <UiButton size="icon" variant="ghost" aria-label="Close" onClick={() => props.onClose()}>
                <PhX size={18} weight="bold" />
              </UiButton>
            </header>
            <div class="min-h-0 flex-1 overflow-auto p-4 sm:p-5">{props.children}</div>
          </div>
        </div>
      </Show>
    </Portal>
  );
}
