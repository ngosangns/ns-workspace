import type { JSX, ParentProps } from "solid-js";
import { Show } from "solid-js";

export default function EmptyState(
  props: ParentProps<{
    title: string;
    description?: string;
    class?: string;
  }>,
): JSX.Element {
  return (
    <div class={`px-5 py-12 text-center ${props.class ?? ""}`}>
      <p class="m-0 mb-1.5 text-[15px] font-semibold text-fg">{props.title}</p>
      <Show when={props.description}>
        <p class="m-0 text-[13px] text-fg-muted">{props.description}</p>
      </Show>
      <Show when={props.children}>
        <div class="mt-4 flex justify-center">{props.children}</div>
      </Show>
    </div>
  );
}
