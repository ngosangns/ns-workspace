import type { JSX, ParentProps } from "solid-js";

export default function UiButton(
  props: ParentProps<{
    variant?: "primary" | "secondary" | "ghost" | "danger" | "warning";
    size?: "sm" | "md" | "icon";
    type?: "button" | "submit" | "reset";
    disabled?: boolean;
    loading?: boolean;
    class?: string;
    "aria-label"?: string;
    onClick?: (e: MouseEvent) => void;
  }>,
): JSX.Element {
  const size = () => props.size ?? "md";
  const variant = () => props.variant ?? "secondary";

  const base =
    "inline-flex items-center justify-center gap-1.5 font-semibold tracking-tight transition duration-160 ease-[var(--ease-out-soft)] disabled:cursor-not-allowed disabled:opacity-50 active:scale-[0.98]";

  const sizes = {
    sm: "min-h-8 rounded-md px-3 text-xs",
    md: "min-h-[34px] rounded-md px-3.5 text-[13px]",
    icon: "h-9 w-9 rounded-full p-0",
  } as const;

  const variants = {
    primary: "bg-accent text-ink hover:bg-accent-hover",
    secondary: "bg-transparent text-fg-secondary hover:bg-hover hover:text-fg",
    ghost: "bg-transparent text-fg-secondary hover:bg-hover hover:text-fg",
    danger: "bg-transparent text-negative hover:bg-negative/10",
    warning: "border border-warning/40 bg-transparent text-warning hover:bg-warning/10",
  } as const;

  return (
    <button
      type={props.type ?? "button"}
      class={`${base} ${sizes[size()]} ${variants[variant()]} ${props.class ?? ""}`}
      disabled={props.disabled || props.loading}
      aria-busy={props.loading || undefined}
      aria-label={props["aria-label"]}
      onClick={props.onClick}
    >
      {props.loading ? (
        <span class="inline-block h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-r-transparent" aria-hidden="true" />
      ) : null}
      {props.children}
    </button>
  );
}
