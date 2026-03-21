"use client";

import { X, FileText, Image as ImageIcon } from "lucide-react";
import type { FileAttachment } from "@/lib/types";

interface AttachmentPreviewProps {
  files: FileAttachment[];
  onRemove: (index: number) => void;
}

export function AttachmentPreview({ files, onRemove }: AttachmentPreviewProps) {
  if (files.length === 0) return null;

  return (
    <div className="flex gap-2 overflow-x-auto px-3 py-2">
      {files.map((file, i) => (
        <div
          key={`${file.name}-${i}`}
          className="group relative flex shrink-0 items-center gap-1.5 rounded-md border border-border bg-muted/50 px-2 py-1 text-xs"
        >
          {file.mimeType.startsWith("image/") ? (
            <ImageIcon className="size-3 text-muted-foreground" />
          ) : (
            <FileText className="size-3 text-muted-foreground" />
          )}
          <span className="max-w-[120px] truncate">{file.name}</span>
          <button
            type="button"
            onClick={() => onRemove(i)}
            className="ml-0.5 rounded p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
            aria-label={`Remove ${file.name}`}
          >
            <X className="size-3" />
          </button>
        </div>
      ))}
    </div>
  );
}
