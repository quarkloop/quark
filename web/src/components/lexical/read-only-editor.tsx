"use client";

import { LexicalComposer } from "@lexical/react/LexicalComposer";
import { RichTextPlugin } from "@lexical/react/LexicalRichTextPlugin";
import { ContentEditable } from "@lexical/react/LexicalContentEditable";
import { HeadingNode, QuoteNode } from "@lexical/rich-text";
import { ListNode, ListItemNode } from "@lexical/list";
import { CodeNode, CodeHighlightNode } from "@lexical/code";
import { LinkNode } from "@lexical/link";
import { $convertFromMarkdownString, TRANSFORMERS } from "@lexical/markdown";
import { editorTheme } from "./editor-theme";

interface ReadOnlyEditorProps {
  content: string;
  className?: string;
}

export function ReadOnlyEditor({ content, className }: ReadOnlyEditorProps) {
  const initialConfig = {
    namespace: "ReadOnly",
    theme: editorTheme,
    editable: false,
    nodes: [
      HeadingNode,
      QuoteNode,
      ListNode,
      ListItemNode,
      CodeNode,
      CodeHighlightNode,
      LinkNode,
    ],
    editorState: () => {
      $convertFromMarkdownString(content, TRANSFORMERS);
    },
    onError: (error: Error) => {
      console.error("Lexical read-only error:", error);
    },
  };

  return (
    <LexicalComposer initialConfig={initialConfig}>
      <RichTextPlugin
        contentEditable={
          <ContentEditable className={className ?? "outline-none"} />
        }
        ErrorBoundary={LexicalErrorBoundary}
      />
    </LexicalComposer>
  );
}

function LexicalErrorBoundary({ children }: { children: React.ReactNode }) {
  return <>{children}</>;
}
