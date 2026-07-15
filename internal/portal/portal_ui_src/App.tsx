import { createSignal, For, Show, type Component } from "solid-js";
import { A, useLocation } from "@solidjs/router";
import { PhSquaresFour, PhBrain, PhHardDrives, PhCirclesFour, PhPuzzlePiece, PhList } from "./components/Icons";

interface NavItem {
  label: string;
  icon: Component<{ size?: number; weight?: "regular" | "bold" | "fill"; class?: string }>;
  to: string;
}

const navItems: NavItem[] = [
  { label: "Dashboard", icon: PhSquaresFour, to: "/" },
  { label: "Skills", icon: PhBrain, to: "/skills" },
  { label: "MCPs", icon: PhHardDrives, to: "/mcps" },
  { label: "Registry", icon: PhCirclesFour, to: "/registry" },
  { label: "Adapters", icon: PhPuzzlePiece, to: "/adapters" },
];

export default function App(props: { children?: any }) {
  const location = useLocation();
  const [drawerOpen, setDrawerOpen] = createSignal(false);

  function isActive(to: string) {
    const path = location.pathname;
    if (to === "/") return path === "/" || path === "";
    return path === to || path.startsWith(`${to}/`);
  }

  function NavButtons(props: { mobile?: boolean }) {
    return (
      <For each={navItems}>
        {(item) => {
          const Icon = item.icon;
          const active = () => isActive(item.to);
          return (
            <A
              href={item.to}
              class={`flex min-h-11 items-center gap-2.5 rounded-md border border-transparent px-3 py-2 text-left text-[13.5px] font-medium text-fg-secondary transition duration-160 ease-[var(--ease-out-soft)] hover:bg-hover hover:text-fg ${active() ? "border-accent-ring bg-accent-soft text-fg" : ""} ${props.mobile ? "" : "w-full gap-[11px] py-2.5 active:scale-[0.99]"}`}
              aria-current={active() ? "page" : undefined}
              onClick={() => setDrawerOpen(false)}
            >
              <Icon size={20} weight={active() ? "fill" : "regular"} class={`shrink-0 ${active() && !props.mobile ? "text-accent" : ""}`} />
              <span>{item.label}</span>
            </A>
          );
        }}
      </For>
    );
  }

  return (
    <div class="min-h-[100dvh] text-fg">
      <a
        href="#main-content"
        class="absolute left-3 top-[-48px] z-[3000] rounded-md bg-accent px-3 py-2 text-[13px] font-semibold text-ink transition-[top] duration-160 ease-[var(--ease-out-soft)] focus:top-3"
      >
        Skip to content
      </a>

      <button
        type="button"
        class="fixed left-3.5 top-3.5 z-[2002] flex h-10 w-10 items-center justify-center rounded-full border border-border bg-surface/90 text-fg-secondary shadow-[var(--shadow-soft)] backdrop-blur-md transition duration-160 ease-[var(--ease-out-soft)] hover:border-border-strong hover:text-fg active:scale-95 lg:hidden"
        aria-label="Open navigation"
        onClick={() => setDrawerOpen(!drawerOpen())}
      >
        <PhList size={20} weight="bold" />
      </button>

      <Show when={drawerOpen()}>
        <div class="fixed inset-0 z-[2001] bg-app/60 backdrop-blur-sm lg:hidden" onClick={() => setDrawerOpen(false)} />
      </Show>
      <aside
        class={`fixed inset-y-0 left-0 z-[2002] flex w-[280px] flex-col border-r border-border bg-surface p-3 transition-transform duration-200 ease-[var(--ease-out-soft)] lg:hidden ${drawerOpen() ? "translate-x-0" : "-translate-x-full"}`}
        aria-label="Mobile navigation"
      >
        <div class="mb-3 flex items-center gap-3 border-b border-border px-2.5 pb-4 pt-2">
          <div
            class="grid h-9 w-9 shrink-0 place-items-center rounded-[10px] border border-border bg-app-muted shadow-[inset_0_1px_0_rgb(255_255_255/0.04)]"
            aria-hidden="true"
          >
            <span class="text-[13px] font-semibold tracking-tight text-accent">ns</span>
          </div>
          <div class="min-w-0">
            <div class="text-sm font-semibold tracking-tight text-fg">ns-workspace</div>
            <div class="text-[11px] font-medium text-fg-muted">Portal</div>
          </div>
        </div>
        <nav class="flex flex-1 flex-col gap-0.5 px-1">
          <NavButtons mobile />
        </nav>
        <div class="mt-2.5 border-t border-border pt-3">
          <div class="flex items-center gap-2 px-3 py-1 text-xs font-medium text-fg-muted">
            <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-accent shadow-[0_0_0_3px_var(--color-accent-soft)]" />
            Local only
          </div>
        </div>
      </aside>

      <aside class="fixed inset-y-0 left-0 z-[2001] hidden w-[248px] p-3.5 lg:block" aria-label="Primary">
        <div class="flex h-full flex-col rounded-xl border border-border bg-surface p-2.5 pt-3.5 shadow-[inset_0_1px_0_rgb(255_255_255/0.04)]">
          <div class="mb-3 flex items-center gap-3 border-b border-border px-2.5 pb-[18px]">
            <div
              class="grid h-9 w-9 shrink-0 place-items-center rounded-[10px] border border-border bg-app-muted shadow-[inset_0_1px_0_rgb(255_255_255/0.04)]"
              aria-hidden="true"
            >
              <span class="text-[13px] font-semibold tracking-tight text-accent">ns</span>
            </div>
            <div class="min-w-0">
              <div class="text-sm font-semibold tracking-tight text-fg">ns-workspace</div>
              <div class="text-[11px] font-medium text-fg-muted">Portal</div>
            </div>
          </div>
          <nav class="flex flex-1 flex-col gap-0.5">
            <NavButtons />
          </nav>
          <div class="mt-2.5 border-t border-border pt-3">
            <div class="flex items-center gap-2 px-3 py-1 text-xs font-medium text-fg-muted">
              <span class="h-1.5 w-1.5 shrink-0 rounded-full bg-accent shadow-[0_0_0_3px_var(--color-accent-soft)]" />
              Local only
            </div>
          </div>
        </div>
      </aside>

      <main id="main-content" class="mx-auto max-w-[1120px] px-4 py-6 outline-none md:px-8 md:py-8 lg:ml-[248px]" tabindex="-1">
        {props.children}
      </main>
    </div>
  );
}
