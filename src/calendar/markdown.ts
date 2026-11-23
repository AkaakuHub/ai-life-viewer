import { toString } from "mdast-util-to-string";
import remarkGfm from "remark-gfm";
import remarkParse from "remark-parse";
import { unified } from "unified";

const parser = unified().use(remarkParse).use(remarkGfm).freeze();

export function plainText(markdown: string) {
  return parser
    .parse(markdown)
    .children.map((node) => {
      const parts = node.type === "list" || node.type === "blockquote" ? node.children : [node];
      return parts.map((part) => toString(part, { includeHtml: false })).join(" ");
    })
    .join(" ")
    .replace(/\s+/g, " ")
    .trim();
}

export function supplementaryPreview(title: string, preview?: string) {
  if (!preview) return undefined;
  const normalized = plainText(preview);
  const collapseWhitespace = (value: string) => value.replace(/\s+/g, " ").trim();
  if (
    normalized &&
    !plainText(title).includes(normalized) &&
    !collapseWhitespace(title).includes(collapseWhitespace(preview))
  )
    return preview;
}
