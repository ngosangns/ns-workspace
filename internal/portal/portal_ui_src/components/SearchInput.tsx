import type { JSX } from "solid-js";
import { PhMagnifyingGlass } from "./Icons";

export default function SearchInput(props: {
  value: string;
  placeholder?: string;
  "aria-label": string;
  class?: string;
  onInput: (value: string) => void;
}): JSX.Element {
  return (
    <label class={`relative min-w-[200px] flex-1 max-w-md ${props.class ?? ""}`}>
      <span class="sr-only">{props["aria-label"]}</span>
      <PhMagnifyingGlass size={16} class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-fg-muted" />
      <input
        type="search"
        class="w-full rounded-md border border-border bg-surface py-2 pl-9 pr-3 text-[13px] text-fg outline-none transition focus:border-accent-ring"
        placeholder={props.placeholder}
        value={props.value}
        onInput={(e) => props.onInput(e.currentTarget.value)}
      />
    </label>
  );
}
