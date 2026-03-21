"use client";

import { useEffect, useRef } from "react";
import { useChat } from "@/hooks/use-chat";
import { useAgentContext } from "@/context/agent-context";
import { ActivityRenderer } from "./activity-renderer";
import { PromptInput } from "./prompt-input";
import { StatusIndicator } from "@/components/layout/status-indicator";
import { MessageSquare } from "lucide-react";

export function ChatContainer() {
  const { activeAgent } = useAgentContext();
  const { activities, isSending, isConnected, error, send } =
    useChat(activeAgent);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [activities]);

  if (!activeAgent) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-4">
        <div className="flex size-14 items-center justify-center rounded-2xl bg-muted">
          <MessageSquare className="size-6 text-muted-foreground" />
        </div>
        <div className="text-center">
          <p className="text-base font-medium text-foreground/70">
            Select an agent to start chatting
          </p>
          <p className="mt-1 text-sm text-muted-foreground">
            Discover or add agents from the sidebar
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      {/* Connection status — only show when disconnected or error */}
      {(!isConnected || error) && (
        <div className="flex items-center gap-2 border-b border-border/40 px-4 py-1.5">
          <StatusIndicator status={isConnected ? "online" : "offline"} />
          <span className="text-xs text-muted-foreground">
            {isConnected ? "Connected" : "Disconnected"}
          </span>
          {error && (
            <span className="ml-auto text-xs text-destructive">{error}</span>
          )}
        </div>
      )}

      {/* Activity stream */}
      <div className="flex-1 overflow-y-auto">
        <div className="mx-auto max-w-3xl py-6">
          {activities.length === 0 ? (
            <p className="py-16 text-center text-sm text-zinc-400">
              Send a message to get started.
            </p>
          ) : (
            <ActivityRenderer activities={activities} />
          )}
          <div ref={bottomRef} />
        </div>
      </div>

      {/* Prompt */}
      <PromptInput onSend={send} disabled={isSending} model={activeAgent?.model} provider={activeAgent?.provider} />
    </div>
  );
}
