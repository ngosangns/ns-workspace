import type { JSX } from "solid-js";

export default function EnableSwitch(props: {
  checked: boolean;
  disabled?: boolean;
  "aria-label": string;
  onChange: (next: boolean) => void;
}): JSX.Element {
  return (
    <label class="inline-flex cursor-pointer items-center gap-2 select-none">
      <span class="text-[11px] font-medium uppercase tracking-wide text-fg-muted">{props.checked ? "On" : "Off"}</span>
      <button
        type="button"
        role="switch"
        aria-checked={props.checked}
        aria-label={props["aria-label"]}
        disabled={props.disabled}
        class={`relative h-5 w-9 shrink-0 rounded-full border transition duration-160 ease-[var(--ease-out-soft)] disabled:cursor-not-allowed disabled:opacity-50 ${
          props.checked ? "border-accent/50 bg-accent" : "border-border bg-app-muted hover:border-border-strong"
        }`}
        onClick={() => {
          if (!props.disabled) props.onChange(!props.checked);
        }}
      >
        <span
          class={`absolute top-0.5 left-0.5 h-3.5 w-3.5 rounded-full bg-fg shadow-sm transition duration-160 ease-[var(--ease-out-soft)] ${
            props.checked ? "translate-x-4 bg-ink" : ""
          }`}
          aria-hidden="true"
        />
      </button>
    </label>
  );
}
