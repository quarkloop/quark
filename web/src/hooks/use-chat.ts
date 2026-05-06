"use client";

import { useState, useCallback, useEffect, useRef, useMemo } from "react";
import type { ActivityRecord, AgentConnection, AgentMode, FileAttachment } from "@/lib/types";
import { useSendMessage } from "@/hooks/use-chat-query";
import { activityKey, useActivity } from "@/hooks/use-activity-query";
import { useQueryClient } from "@tanstack/react-query";
import { BASE } from "@/hooks/http";

export function useChat(agent: AgentConnection | undefined, sessionKey?: string | null) {
  const [sseActivities, setSseActivities] = useState<ActivityRecord[]>([]);
  const [isConnected, setIsConnected] = useState(false);
  const eventSourceRef = useRef<EventSource | null>(null);
  const seenIdsRef = useRef(new Set<string>());
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const queryClient = useQueryClient();

  const sendMut = useSendMessage(agent?.id, agent?.baseUrl, agent?.spaceId);
  const { data: history = [] } = useActivity(
    agent?.id,
    agent?.baseUrl,
    sessionKey,
    agent?.spaceId,
  );

  // Merge history + SSE activities.
  const allActivities = useMemo(() => {
    const merged = [...history];
    for (const a of sseActivities) {
      if (!seenIdsRef.current.has(a.id)) {
        seenIdsRef.current.add(a.id);
        merged.push(a);
      }
    }
    return merged;
  }, [history, sseActivities]);

  // Seed seenIds from history.
  useEffect(() => {
    for (const a of history) {
      seenIdsRef.current.add(a.id);
    }
  }, [history]);

  // SSE stream with reconnect.
  useEffect(() => {
    if (!agent || !sessionKey) {
      setSseActivities([]);
      setIsConnected(false);
      seenIdsRef.current.clear();
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
      return;
    }

    let cancelled = false;
    let attempt = 0;

    const connect = () => {
      if (cancelled) return;

      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }

      const es = new EventSource(
        `/api/v1/agents/${agent.id}/sessions/${encodeURIComponent(sessionKey)}/activity/stream?${BASE(agent.baseUrl, agent.spaceId)}`,
      );
      eventSourceRef.current = es;

      es.onopen = () => {
        if (!cancelled) {
          attempt = 0;
          setIsConnected(true);
        }
      };

      es.onmessage = (event) => {
        if (cancelled) return;
        try {
          const record: ActivityRecord = JSON.parse(event.data);
          if (!seenIdsRef.current.has(record.id)) {
            seenIdsRef.current.add(record.id);
            setSseActivities((prev) => [...prev, record]);
          }
        } catch {
          // Ignore parse errors.
        }
      };

      es.onerror = () => {
        if (cancelled) return;
        setIsConnected(false);
        es.close();
        eventSourceRef.current = null;

        const scheduleReconnect = () => {
          if (cancelled) return;
          const delay = Math.min(1000 * Math.pow(2, attempt), 10000);
          attempt++;
          reconnectTimerRef.current = setTimeout(() => {
            if (cancelled) return;
            connect();
          }, delay);
        };
        scheduleReconnect();
      };
    };

    connect();

    return () => {
      cancelled = true;
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
    };
  }, [agent, sessionKey]);

  // Filter activities by session key.
  const activities = useMemo(() => {
    if (!sessionKey) return allActivities;
    return allActivities.filter(
      (a) => a.session_id === sessionKey || a.session_id === "" || !a.session_id,
    );
  }, [allActivities, sessionKey]);

  const send = useCallback(
    (message: string, mode: AgentMode = "ask", files?: FileAttachment[]) => {
      if (!agent || !sessionKey || !message.trim()) return;
      sendMut.mutate(
        { message, mode, files, sessionKey },
        {
          onSuccess: () => {
            queryClient.invalidateQueries({
              queryKey: activityKey(agent.id, sessionKey),
            });
          },
        },
      );
    },
    [agent, sessionKey, sendMut, queryClient],
  );

  return {
    activities,
    isSending: sendMut.isPending,
    isConnected,
    error: sendMut.error?.message ?? null,
    clearError: () => sendMut.reset(),
    send,
  };
}
