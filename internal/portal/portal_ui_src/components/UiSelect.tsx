import type { JSX } from "solid-js";
import { Index, createEffect, on } from "solid-js";

export interface SelectOption {
  label: string;
  value: string;
}

/**
 * Controlled native select.
 *
 * Solid's keyed <For> remounts <option> nodes when the options array is
 * recreated (label/count updates). The browser then resets selection to the
 * first option and may fire a synthetic change event — which looked like the
 * registry filter "resetting" after catalog refetch.
 *
 * Mitigations:
 * - <Index> keeps option nodes stable by position (label text updates in place)
 * - Re-apply props.value after option updates
 * - Ignore change events while applying programmatic value
 */
export default function UiSelect(props: {
  value: string;
  options: SelectOption[];
  disabled?: boolean;
  id?: string;
  onChange: (value: string) => void;
}): JSX.Element {
  let selectEl: HTMLSelectElement | undefined;
  let ignoreChangeUntil = 0;

  function applyValue() {
    const el = selectEl;
    if (!el) return;
    const value = props.value;
    if (el.value === value) return;
    ignoreChangeUntil = Date.now() + 100;
    el.value = value;
  }

  createEffect(
    on(
      () => [props.value, props.options.map((o) => `${o.value}\0${o.label}`).join("\n")] as const,
      () => {
        queueMicrotask(applyValue);
      },
    ),
  );

  return (
    <select
      ref={(el) => {
        selectEl = el;
        queueMicrotask(applyValue);
      }}
      id={props.id}
      value={props.value}
      disabled={props.disabled}
      class="min-w-[160px] flex-1 appearance-none rounded-md border border-border bg-app-muted px-3 py-2 pr-8 text-[13px] font-medium text-fg outline-none transition duration-160 ease-[var(--ease-out-soft)] hover:border-border-strong focus:border-accent disabled:cursor-not-allowed disabled:opacity-50"
      onChange={(e) => {
        if (Date.now() < ignoreChangeUntil) {
          applyValue();
          return;
        }
        const next = e.currentTarget.value;
        if (next === props.value) return;
        props.onChange(next);
      }}
    >
      <Index each={props.options}>{(opt) => <option value={opt().value}>{opt().label}</option>}</Index>
    </select>
  );
}
