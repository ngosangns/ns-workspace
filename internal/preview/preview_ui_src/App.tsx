import { A, useLocation } from "@solidjs/router";
import type { ParentProps } from "solid-js";

const nav = [
  { to: "/", label: "Docs" },
  { to: "/search", label: "Search" },
  { to: "/graph", label: "Graph" },
];

export default function App(props: ParentProps) {
  const location = useLocation();
  function active(to: string) {
    if (to === "/") return location.pathname === "/" || location.pathname.startsWith("/docs");
    return location.pathname === to || location.pathname.startsWith(`${to}/`);
  }

  return (
    <div class="flex min-h-full flex-col">
      <header class="flex flex-wrap items-center gap-4 border-b border-border bg-surface px-4 py-3">
        <div class="flex items-center gap-2">
          <span class="grid h-8 w-8 place-items-center rounded-lg border border-border bg-app-muted text-xs font-semibold text-accent">
            ns
          </span>
          <div>
            <div class="text-sm font-semibold tracking-tight">ns-workspace</div>
            <div class="text-[11px] text-fg-muted">Docs preview</div>
          </div>
        </div>
        <nav class="flex gap-1">
          {nav.map((item) => (
            <A
              href={item.to}
              class={`rounded-md px-3 py-1.5 text-[13px] font-medium transition ${
                active(item.to) ? "bg-accent-soft text-fg" : "text-fg-secondary hover:bg-hover hover:text-fg"
              }`}
            >
              {item.label}
            </A>
          ))}
        </nav>
      </header>
      <main class="mx-auto w-full max-w-[1200px] flex-1 px-4 py-5">{props.children}</main>
    </div>
  );
}
