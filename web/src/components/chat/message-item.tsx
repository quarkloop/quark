"use client";

import { ReadOnlyEditor } from "@/components/lexical/read-only-editor";

interface MessageItemProps {
  author: string;
  content: string;
  timestamp: string;
}

export function MessageItem({ author, content, timestamp }: MessageItemProps) {
  const isUser = author === "user";
  const time = formatTime(timestamp);

  if (isUser) {
    return (
      <div className="flex justify-end px-5 py-2">
        <div className="max-w-[70%]">
          <div className="rounded-2xl rounded-br-sm bg-zinc-900 px-4 py-2.5 text-[14px] leading-relaxed text-white">
            <p className="whitespace-pre-wrap">{content}</p>
          </div>
          <p className="mt-0.5 pr-1 text-right text-[11px] text-zinc-300">
            {time}
          </p>
        </div>
      </div>
    );
  }

  // Agent message — clean, no bubble
  return (
    <div className="px-5 py-2">
      <div className="max-w-[85%] text-[14px] leading-[1.7] text-zinc-800 [&_strong]:font-semibold [&_code]:rounded [&_code]:bg-zinc-100 [&_code]:px-1 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-[13px] [&_code]:text-zinc-700 [&_pre]:my-2 [&_pre]:rounded-lg [&_pre]:bg-zinc-900 [&_pre]:px-3 [&_pre]:py-2 [&_pre]:font-mono [&_pre]:text-[13px] [&_pre]:text-zinc-300">
        <ReadOnlyEditor content={content} />
        <p className="mt-0.5 text-[11px] text-zinc-300">{time}</p>
      </div>
    </div>
  );
}

function formatTime(ts: string) {
  try {
    return new Date(ts).toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
    });
  } catch {
    return "";
  }
}
