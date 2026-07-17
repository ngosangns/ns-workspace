import type { JSX, ParentProps } from "solid-js";

export type StatusPillKind = "ok" | "err" | "warn" | "muted" | "accent";

const kindClass: Record<StatusPillKind, string> = {
  ok: "status-pill--ok",
  err: "status-pill--err",
  warn: "status-pill--warn",
  muted: "status-pill--muted",
  accent: "status-pill--accent",
};

export default function StatusPill(
  props: ParentProps<{
    kind?: StatusPillKind;
    class?: string;
  }>,
): JSX.Element {
  const kind = () => props.kind ?? "muted";
  return <span class={`status-pill ${kindClass[kind()]} ${props.class ?? ""}`}>{props.children}</span>;
}
