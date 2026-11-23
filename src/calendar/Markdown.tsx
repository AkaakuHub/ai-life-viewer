import { Typography } from "@mantine/core";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

export function Markdown({ children }: { children: string }) {
  return (
    <Typography style={{ minWidth: 0, overflowWrap: "anywhere" }}>
      <ReactMarkdown skipHtml remarkPlugins={[remarkGfm]} components={{ img: ({ alt }) => alt }}>
        {children}
      </ReactMarkdown>
    </Typography>
  );
}
