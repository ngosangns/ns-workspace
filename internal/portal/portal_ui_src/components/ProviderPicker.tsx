import { For, Show, createEffect, createMemo, createSignal, onCleanup, type JSX } from "solid-js";
import { Portal } from "solid-js/web";

export interface ProviderOption {
  id: string;
  label: string;
}

const MENU_ATTR = "data-provider-picker-menu";

/** Unrestricted (missing / ["all"]). */
function isAll(providers: string[] | undefined): boolean {
  if (providers == null) return true;
  return providers.some((p) => p === "all" || p === "*");
}

/** Explicit empty allowlist — no providers. */
function isNone(providers: string[] | undefined): boolean {
  return Array.isArray(providers) && providers.length === 0;
}

function labelFor(providers: string[] | undefined, options: ProviderOption[]): string {
  if (isAll(providers)) return "All providers";
  if (isNone(providers)) return "No providers";
  const ids = providers ?? [];
  if (ids.length === 1) {
    const opt = options.find((o) => o.id === ids[0]);
    return opt?.label ?? ids[0];
  }
  return `${ids.length} providers`;
}

/**
 * Compact multi-select for allowed providers. Default is "all".
 * - All ON  → ["all"], every provider checkbox checked
 * - All OFF → [], every provider checkbox unchecked
 * - Partial → selected ids; All unchecked (indeterminate when some selected)
 */
export default function ProviderPicker(props: {
  value: string[] | undefined;
  options: ProviderOption[];
  disabled?: boolean;
  "aria-label"?: string;
  onChange: (next: string[]) => void;
}): JSX.Element {
  const [open, setOpen] = createSignal(false);
  const [menuStyle, setMenuStyle] = createSignal<JSX.CSSProperties>({
    position: "fixed",
    top: "0px",
    left: "0px",
    visibility: "hidden",
    "z-index": 80,
  });
  let buttonEl: HTMLButtonElement | undefined;
  let menuEl: HTMLDivElement | undefined;
  let allInputEl: HTMLInputElement | undefined;

  const all = createMemo(() => isAll(props.value));
  const none = createMemo(() => isNone(props.value));
  const selected = createMemo(() => {
    if (all() || none()) return new Set<string>();
    return new Set((props.value ?? []).map((p) => p.toLowerCase()));
  });
  const selectedCount = createMemo(() => {
    if (all()) return props.options.length;
    if (none()) return 0;
    return selected().size;
  });
  const summary = createMemo(() => labelFor(props.value, props.options));

  // Indeterminate All when partial selection.
  createEffect(() => {
    const el = allInputEl;
    if (!el) return;
    const partial = !all() && !none() && selectedCount() > 0 && selectedCount() < props.options.length;
    el.indeterminate = partial;
  });

  function placeMenu() {
    const btn = buttonEl;
    const menu = menuEl;
    if (!btn || !menu) return;

    const rect = btn.getBoundingClientRect();
    const menuWidth = Math.max(menu.offsetWidth || 192, 192);
    const menuHeight = menu.offsetHeight || 200;
    const gap = 4;
    const pad = 8;

    let left = rect.right - menuWidth;
    left = Math.min(left, window.innerWidth - menuWidth - pad);
    left = Math.max(pad, left);

    const spaceBelow = window.innerHeight - rect.bottom - gap - pad;
    const spaceAbove = rect.top - gap - pad;
    let top: number;
    if (spaceBelow >= menuHeight || spaceBelow >= spaceAbove) {
      top = rect.bottom + gap;
    } else {
      top = rect.top - gap - menuHeight;
    }
    top = Math.max(pad, Math.min(top, window.innerHeight - menuHeight - pad));

    setMenuStyle({
      position: "fixed",
      top: `${Math.round(top)}px`,
      left: `${Math.round(left)}px`,
      "min-width": "12rem",
      "max-height": `${Math.min(320, window.innerHeight - pad * 2)}px`,
      "z-index": 80,
      visibility: "visible",
    });
  }

  /** All checkbox: checked → all providers; unchecked → none. */
  function onAllChange(checked: boolean) {
    if (checked) {
      props.onChange(["all"]);
    } else {
      props.onChange([]);
    }
  }

  function toggle(id: string) {
    let next: Set<string>;
    if (all()) {
      // Leave all-mode: every provider except the one toggled off.
      next = new Set(props.options.map((o) => o.id).filter((x) => x !== id));
    } else if (none()) {
      // Leave none-mode: only the one toggled on.
      next = new Set([id]);
    } else {
      next = new Set(selected());
      if (next.has(id)) next.delete(id);
      else next.add(id);
    }
    if (next.size === 0) {
      props.onChange([]);
      return;
    }
    if (next.size === props.options.length) {
      props.onChange(["all"]);
      return;
    }
    props.onChange([...next].sort());
  }

  function isChecked(id: string): boolean {
    if (all()) return true;
    if (none()) return false;
    return selected().has(id);
  }

  function eventInsidePicker(e: Event): boolean {
    const path = typeof e.composedPath === "function" ? e.composedPath() : [];
    for (const node of path) {
      if (node === buttonEl || node === menuEl) return true;
      if (node instanceof Element && node.hasAttribute(MENU_ATTR)) return true;
    }
    const t = e.target;
    if (t instanceof Node) {
      if (buttonEl?.contains(t) || menuEl?.contains(t)) return true;
      if (t instanceof Element && t.closest(`[${MENU_ATTR}]`)) return true;
    }
    return false;
  }

  createEffect(() => {
    if (!open()) return;

    const raf = requestAnimationFrame(() => placeMenu());

    const onReposition = () => placeMenu();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };

    let removeOutside: (() => void) | undefined;
    const outsideTimer = window.setTimeout(() => {
      const onPointerDown = (e: PointerEvent) => {
        if (eventInsidePicker(e)) return;
        setOpen(false);
      };
      document.addEventListener("pointerdown", onPointerDown);
      removeOutside = () => document.removeEventListener("pointerdown", onPointerDown);
    }, 0);

    document.addEventListener("keydown", onKey);
    window.addEventListener("resize", onReposition);
    window.addEventListener("scroll", onReposition, true);

    onCleanup(() => {
      cancelAnimationFrame(raf);
      window.clearTimeout(outsideTimer);
      removeOutside?.();
      document.removeEventListener("keydown", onKey);
      window.removeEventListener("resize", onReposition);
      window.removeEventListener("scroll", onReposition, true);
    });
  });

  return (
    <div class="relative inline-flex">
      <button
        ref={(el) => {
          buttonEl = el;
        }}
        type="button"
        disabled={props.disabled}
        aria-label={props["aria-label"] ?? "Allowed providers"}
        aria-expanded={open()}
        aria-haspopup="listbox"
        class="inline-flex max-w-[11rem] items-center gap-1 truncate rounded-md border border-border bg-app-muted px-2 py-1 text-[11.5px] font-medium text-fg outline-none transition duration-160 ease-[var(--ease-out-soft)] hover:border-border-strong focus:border-accent disabled:cursor-not-allowed disabled:opacity-50"
        onClick={(e) => {
          e.stopPropagation();
          e.preventDefault();
          if (props.disabled) return;
          setOpen((v) => !v);
        }}
      >
        <span class="truncate">{summary()}</span>
        <span class="text-fg-muted" aria-hidden="true">
          ▾
        </span>
      </button>
      <Show when={open()}>
        <Portal>
          <div
            ref={(el) => {
              menuEl = el;
              if (el) requestAnimationFrame(() => placeMenu());
            }}
            data-provider-picker-menu=""
            role="listbox"
            aria-multiselectable="true"
            aria-label={props["aria-label"] ?? "Allowed providers"}
            class="overflow-y-auto rounded-md border border-border bg-elevated p-1.5 shadow-lg"
            style={menuStyle()}
            onClick={(e) => e.stopPropagation()}
            onPointerDown={(e) => e.stopPropagation()}
            onMouseDown={(e) => e.stopPropagation()}
          >
            <label class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-[12.5px] text-fg hover:bg-app-muted">
              <input
                ref={(el) => {
                  allInputEl = el;
                }}
                type="checkbox"
                checked={all()}
                disabled={props.disabled}
                class="accent-[var(--color-accent)]"
                onChange={(e) => {
                  e.stopPropagation();
                  onAllChange(e.currentTarget.checked);
                }}
                onClick={(e) => e.stopPropagation()}
              />
              <span class="font-medium">All providers</span>
            </label>
            <div class="my-1 border-t border-border" />
            <For each={props.options}>
              {(opt) => (
                <label class="flex cursor-pointer items-center gap-2 rounded px-2 py-1.5 text-[12.5px] text-fg hover:bg-app-muted">
                  <input
                    type="checkbox"
                    checked={isChecked(opt.id)}
                    disabled={props.disabled}
                    class="accent-[var(--color-accent)]"
                    onChange={(e) => {
                      e.stopPropagation();
                      toggle(opt.id);
                    }}
                    onClick={(e) => e.stopPropagation()}
                  />
                  <span class="truncate font-mono text-[12px]">{opt.label}</span>
                </label>
              )}
            </For>
          </div>
        </Portal>
      </Show>
    </div>
  );
}
