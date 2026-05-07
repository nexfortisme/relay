import DOMPurify from "dompurify";
import { marked } from "marked";

marked.setOptions({
  gfm: true,
  breaks: true,
});

const markdownRenderer = new marked.Renderer();
markdownRenderer.code = ({ text, lang }: { text: string; lang?: string }) => {
  const language = typeof lang === "string" && lang.trim() ? lang.trim() : "";
  const safeLanguage = escapeHtml(language);
  const codeClass = safeLanguage ? ` class="language-${safeLanguage}"` : "";
  return [
    '<div class="code-block">',
    '<button type="button" class="code-copy-button" aria-label="Copy code">Copy</button>',
    `<pre><code${codeClass}>${escapeHtml(text)}</code></pre>`,
    "</div>",
  ].join("");
};

marked.use({ renderer: markdownRenderer });

// Open external citation links in a new tab. Only absolute http(s) URLs get
// target=_blank; relative links and in-page fragments still navigate in place.
DOMPurify.addHook("afterSanitizeAttributes", (node) => {
  if (!(node instanceof HTMLAnchorElement)) {
    return;
  }
  const href = node.getAttribute("href") ?? "";
  if (!/^https?:\/\//i.test(href)) {
    return;
  }
  node.setAttribute("target", "_blank");
  node.setAttribute("rel", "noopener noreferrer");
});

export function renderMarkdown(content: string): string {
  const parsed = marked.parse(content, { async: false });
  return DOMPurify.sanitize(parsed);
}

function escapeHtml(value: string): string {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}
