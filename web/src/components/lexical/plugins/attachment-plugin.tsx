"use client";

import { useEffect } from "react";
import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";

interface AttachmentPluginProps {
  onFilesAdded: (files: File[]) => void;
}

export function AttachmentPlugin({ onFilesAdded }: AttachmentPluginProps) {
  const [editor] = useLexicalComposerContext();

  useEffect(() => {
    const root = editor.getRootElement();
    if (!root) return;

    const handleDrop = (e: DragEvent) => {
      e.preventDefault();
      const files = Array.from(e.dataTransfer?.files ?? []);
      if (files.length > 0) onFilesAdded(files);
    };

    const handleDragOver = (e: DragEvent) => {
      e.preventDefault();
    };

    const handlePaste = (e: ClipboardEvent) => {
      const files = Array.from(e.clipboardData?.files ?? []);
      if (files.length > 0) {
        e.preventDefault();
        onFilesAdded(files);
      }
    };

    root.addEventListener("drop", handleDrop);
    root.addEventListener("dragover", handleDragOver);
    root.addEventListener("paste", handlePaste);

    return () => {
      root.removeEventListener("drop", handleDrop);
      root.removeEventListener("dragover", handleDragOver);
      root.removeEventListener("paste", handlePaste);
    };
  }, [editor, onFilesAdded]);

  return null;
}
