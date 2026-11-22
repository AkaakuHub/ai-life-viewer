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
