import type { JSX } from "solid-js";
import { For } from "solid-js";

export interface SelectOption {
  label: string;
  value: string;
}

export default function UiSelect(props: {
  value: string;
  options: SelectOption[];
  disabled?: boolean;
  id?: string;
  onChange: (value: string) => void;
}): JSX.Element {
  return (
    <select
      id={props.id}
      value={props.value}
      disabled={props.disabled}
      class="min-w-[160px] flex-1 appearance-none rounded-md border border-border bg-app-muted px-3 py-2 pr-8 text-[13px] font-medium text-fg outline-none transition duration-160 ease-[var(--ease-out-soft)] hover:border-border-strong focus:border-accent disabled:cursor-not-allowed disabled:opacity-50"
      onChange={(e) => props.onChange(e.currentTarget.value)}
    >
      <For each={props.options}>{(opt) => <option value={opt.value}>{opt.label}</option>}</For>
    </select>
  );
}
