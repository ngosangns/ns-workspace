import type { JSX, ParentProps } from "solid-js";
import { Show } from "solid-js";

export default function PageHeader(
  props: ParentProps<{
    title: string;
    subtitle?: string;
    class?: string;
  }>,
): JSX.Element {
  return (
    <header class={`page-header fade-in-up is-visible ${props.class ?? ""}`}>
      <h1 class="page-title">{props.title}</h1>
      <Show when={props.subtitle}>
        <p class="page-subtitle">{props.subtitle}</p>
      </Show>
      {props.children}
    </header>
  );
}
