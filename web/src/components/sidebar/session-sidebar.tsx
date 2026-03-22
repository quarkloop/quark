"use client";

import { useEffect, useCallback } from "react";
import { useAgentContext } from "@/context/agent-context";
import { getSessions, createSession, deleteSession } from "@/lib/api-client";
import { Button } from "@/components/ui/button";
import { Plus, Trash2, MessageCircle, Bot } from "lucide-react";

export function SessionSidebar() {
  const { state, dispatch, activeAgent, activeSession } = useAgentContext();
  const { sessions } = state;

  // Load sessions when agent changes.
  useEffect(() => {
    if (!activeAgent) return;
    let cancelled = false;
    getSessions(activeAgent.id, activeAgent.baseUrl)
      .then((sessions) => {
        if (!cancelled) dispatch({ type: "SET_SESSIONS", sessions });
      })
      .catch(() => {});
    return () => { cancelled = true; };
  }, [activeAgent, dispatch]);

  const handleCreate = useCallback(async () => {
    if (!activeAgent) return;
    try {
      const resp = await createSession(activeAgent.id, activeAgent.baseUrl, {
        type: "chat",
      });
      dispatch({ type: "ADD_SESSION", session: resp.session });
    } catch {
      // Ignore errors for now.
    }
  }, [activeAgent, dispatch]);

  const handleDelete = useCallback(
    async (key: string) => {
      if (!activeAgent) return;
      try {
        await deleteSession(activeAgent.id, activeAgent.baseUrl, key);
        dispatch({ type: "REMOVE_SESSION", key });
      } catch {
        // Ignore errors for now.
      }
    },
    [activeAgent, dispatch],
  );

  if (!activeAgent) return null;

  // Only show main and chat sessions (not subagent/cron).
  const visible = sessions.filter(
    (s) => s.type === "main" || s.type === "chat",
  );

  return (
    <div className="flex flex-col border-t border-border/60 px-2 py-2">
      <div className="flex items-center justify-between px-2 pb-1">
        <span className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">
          Sessions
        </span>
        <Button
          variant="ghost"
          size="icon-xs"
          onClick={handleCreate}
          aria-label="New chat session"
          className="size-6 text-muted-foreground hover:text-foreground"
        >
          <Plus className="size-3" />
        </Button>
      </div>
      <div className="space-y-0.5">
        {visible.map((session) => (
          <div
            key={session.key}
            className={`group flex items-center gap-2 rounded-md px-2 py-1.5 text-sm cursor-pointer transition-colors ${
              activeSession?.key === session.key
                ? "bg-accent text-accent-foreground"
                : "text-muted-foreground hover:bg-muted hover:text-foreground"
            }`}
            onClick={() =>
              dispatch({ type: "SET_ACTIVE_SESSION", key: session.key })
            }
          >
            {session.type === "main" ? (
              <Bot className="size-3.5 shrink-0" />
            ) : (
              <MessageCircle className="size-3.5 shrink-0" />
            )}
            <span className="flex-1 truncate text-xs">
              {session.title || (session.type === "main" ? "Main" : "Chat")}
            </span>
            {session.type !== "main" && (
              <Button
                variant="ghost"
                size="icon-xs"
                onClick={(e) => {
                  e.stopPropagation();
                  handleDelete(session.key);
                }}
                className="size-5 opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-destructive"
                aria-label="Delete session"
              >
                <Trash2 className="size-3" />
              </Button>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
