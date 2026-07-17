import type { JSX } from "solid-js";
import { Show } from "solid-js";
import AppAlert from "./AppAlert";

export default function PageFeedback(props: { error?: string; success?: string; class?: string }): JSX.Element {
  return (
    <Show when={props.error || props.success}>
      <div class={`space-y-2 px-4 pt-3 ${props.class ?? ""}`}>
        <Show when={props.error}>
          <AppAlert kind="error" class="!mb-0">
            {props.error}
          </AppAlert>
        </Show>
        <Show when={props.success}>
          <AppAlert kind="success" class="!mb-0">
            {props.success}
          </AppAlert>
        </Show>
      </div>
    </Show>
  );
}
