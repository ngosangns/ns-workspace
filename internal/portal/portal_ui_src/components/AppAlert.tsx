import type { JSX, ParentProps } from "solid-js";
import { Show } from "solid-js";

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

export default function AppAlert(
  props: ParentProps<{
    kind?: "error" | "success" | "info" | "warning";
    title?: string;
    class?: string;
  }>,
): JSX.Element {
  const kind = () => props.kind ?? "error";
  return (
    <Show when={props.children}>
      <div class={`mb-4 rounded-md border px-3.5 py-3 ${kindClass[kind()]} ${props.class ?? ""}`} role="alert">
        <Show when={props.title}>
          <div class={`mb-0.5 text-[13px] font-semibold tracking-tight ${titleClass[kind()]}`}>{props.title}</div>
        </Show>
        <div class="text-[13px] leading-snug text-fg-secondary">{props.children}</div>
      </div>
    </Show>
  );
}
