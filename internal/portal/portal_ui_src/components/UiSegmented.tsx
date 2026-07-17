import type { JSX } from "solid-js";
import { For } from "solid-js";

export interface SegmentedOption<T extends string = string> {
  value: T;
  label: string;
}

export default function UiSegmented<T extends string>(props: {
  value: T;
  options: SegmentedOption<T>[];
  disabled?: boolean;
  "aria-label"?: string;
  onChange: (value: T) => void;
}): JSX.Element {
  return (
    <div class="inline-flex gap-0.5 rounded-md border border-border bg-app-muted p-0.5" role="tablist" aria-label={props["aria-label"]}>
      <For each={props.options}>
        {(opt) => (
          <button
            type="button"
            role="tab"
            aria-selected={props.value === opt.value}
            disabled={props.disabled}
            class={`rounded-[5px] px-3 py-1.5 text-[13px] font-semibold transition duration-160 ease-[var(--ease-out-soft)] disabled:cursor-not-allowed disabled:opacity-50 ${
              props.value === opt.value ? "bg-surface text-fg shadow-sm" : "text-fg-secondary hover:text-fg"
            }`}
            onClick={() => props.onChange(opt.value)}
          >
            {opt.label}
          </button>
        )}
      </For>
    </div>
  );
}
