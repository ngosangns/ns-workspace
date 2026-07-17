import type { JSX, ParentProps } from "solid-js";

/** Flat list row for resource pages (skills, adapters, registry, …). */
export default function ResourceRow(
  props: ParentProps<{
    /** When false, row appears dimmed (disabled resource). */
    enabled?: boolean;
    class?: string;
  }>,
): JSX.Element {
  const enabled = () => props.enabled !== false;
  return (
    <li
      class={`flex flex-wrap items-start gap-x-4 gap-y-2 px-4 py-3 transition duration-160 ease-[var(--ease-out-soft)] hover:bg-elevated ${
        enabled() ? "" : "opacity-60"
      } ${props.class ?? ""}`}
    >
      {props.children}
    </li>
  );
}
