import type { JSX } from "solid-js";
import { For } from "solid-js";

export default function ListSkeleton(props: { rows?: number; rowClass?: string; "aria-label"?: string }): JSX.Element {
  const n = () => props.rows ?? 6;
  return (
    <div class="space-y-0 divide-y divide-border p-0" aria-busy="true" aria-label={props["aria-label"]}>
      <For each={Array.from({ length: n() }, (_, i) => i)}>
        {() => (
          <div class="px-4 py-3">
            <div class={`skeleton h-12 rounded-md ${props.rowClass ?? ""}`} />
          </div>
        )}
      </For>
    </div>
  );
}
