export function escapeHTML(str: string): string {
  return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#039;");
}

export function renderCodePreview(raw: string, language = "text"): string {
  const lang = language ? ` data-source-language="${escapeHTML(language)}"` : "";
  return `<pre class="code-preview bg-base-300 rounded p-4 overflow-auto"><code${lang}>${escapeHTML(raw)}</code></pre>`;
}

export function decorateCodePreviewLines(root: HTMLElement): void {
  root.querySelectorAll<HTMLElement>("pre code").forEach((code) => {
    if (code.dataset.lines === "yes") return;
    const lines = code.textContent?.split("\n") || [];
    code.innerHTML = lines
      .map(
        (line, index) =>
          `<span class="code-line" data-line="${index + 1}"><span class="code-line-number">${index + 1}</span><span class="code-line-content">${escapeHTML(line || " ")}</span></span>`,
      )
      .join("");
    code.dataset.lines = "yes";
  });
}

export function scrollPreviewToLine(root: HTMLElement, line: number): void {
  if (!line) return;
  window.setTimeout(() => {
    const target = root.querySelector<HTMLElement>(`[data-line="${line}"]`);
    if (!target) return;
    target.classList.add("code-line-target");
    target.scrollIntoView({ block: "center" });
  }, 40);
}
