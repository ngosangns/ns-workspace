import { createEffect, onCleanup, onMount } from "solid-js";
import { EditorView, keymap, placeholder as placeholderExt } from "@codemirror/view";
import { EditorState, Compartment, type Extension } from "@codemirror/state";
import { indentWithTab } from "@codemirror/commands";
import { json } from "@codemirror/lang-json";
import { markdown } from "@codemirror/lang-markdown";
import { oneDark } from "@codemirror/theme-one-dark";
import { linter, lintGutter } from "@codemirror/lint";

export default function CodeEditor(props: {
  value: string;
  onChange?: (value: string) => void;
  lang?: "json" | "markdown" | "text";
  readonly?: boolean;
  dark?: boolean;
  placeholder?: string;
}) {
  let host!: HTMLDivElement;
  let view: EditorView | null = null;

  const readOnlyCompartment = new Compartment();
  const editableCompartment = new Compartment();
  const languageCompartment = new Compartment();
  const themeCompartment = new Compartment();

  const jsonLinter = linter((v) => {
    const doc = v.state.doc.toString();
    try {
      JSON.parse(doc);
      return [];
    } catch (err) {
      const message = err instanceof Error ? err.message : "Invalid JSON";
      const match = message.match(/position (\d+)/);
      const pos = match ? Number.parseInt(match[1], 10) : 0;
      const line = v.state.doc.lineAt(Math.min(pos, v.state.doc.length));
      return [{ from: line.from, to: line.to, severity: "error" as const, message }];
    }
  });

  function languageExtension(): Extension[] {
    switch (props.lang ?? "text") {
      case "json":
        return [json(), jsonLinter, lintGutter()];
      case "markdown":
        return [markdown()];
      default:
        return [];
    }
  }

  function themeExtension(): Extension[] {
    return (props.dark ?? true) ? [oneDark] : [];
  }

  onMount(() => {
    const extensions: Extension[] = [
      readOnlyCompartment.of(EditorState.readOnly.of(!!props.readonly)),
      editableCompartment.of(EditorView.editable.of(!props.readonly)),
      languageCompartment.of(languageExtension()),
      themeCompartment.of(themeExtension()),
      keymap.of([indentWithTab]),
      EditorView.updateListener.of((update) => {
        if (update.docChanged) {
          const value = update.state.doc.toString();
          if (value !== props.value) props.onChange?.(value);
        }
      }),
      EditorView.theme({
        "&": { minHeight: "500px", background: "var(--color-app)", fontSize: "13px" },
        "&.cm-focused": { outline: "none" },
        ".cm-scroller": { fontFamily: "var(--font-mono, monospace)", fontVariantNumeric: "tabular-nums" },
        ".cm-gutters": { borderRight: "1px solid var(--color-border)" },
      }),
    ];
    if (props.placeholder) extensions.push(placeholderExt(props.placeholder));

    view = new EditorView({
      doc: props.value,
      extensions,
      parent: host,
    });
  });

  createEffect(() => {
    const value = props.value;
    if (!view) return;
    const current = view.state.doc.toString();
    if (value === current) return;
    view.dispatch({ changes: { from: 0, to: current.length, insert: value } });
  });

  createEffect(() => {
    const readonly = !!props.readonly;
    view?.dispatch({
      effects: [
        readOnlyCompartment.reconfigure(EditorState.readOnly.of(readonly)),
        editableCompartment.reconfigure(EditorView.editable.of(!readonly)),
      ],
    });
  });

  createEffect(() => {
    const dark = props.dark ?? true;
    view?.dispatch({ effects: themeCompartment.reconfigure(dark ? [oneDark] : []) });
  });

  createEffect(() => {
    props.lang;
    view?.dispatch({ effects: languageCompartment.reconfigure(languageExtension()) });
  });

  onCleanup(() => {
    view?.destroy();
    view = null;
  });

  return (
    <div
      ref={host}
      class={`code-editor overflow-hidden font-mono text-[13px] leading-[1.6] ${props.readonly ? "code-editor--readonly" : ""}`}
    />
  );
}
