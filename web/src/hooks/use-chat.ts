"use client";

import { useState, useCallback, useEffect, useRef, useMemo } from "react";
import type { ActivityRecord, AgentConnection, AgentMode, FileAttachment } from "@/lib/types";
import {
  sendMessage,
  getActivity,
  createActivityStream,
} from "@/lib/api-client";

export function useChat(agent: AgentConnection | undefined, sessionKey?: string | null) {
  const [allActivities, setAllActivities] = useState<ActivityRecord[]>([]);
  const [isSending, setIsSending] = useState(false);
  const [isConnected, setIsConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const seenIdsRef = useRef(new Set<string>());

  // Load history and start SSE stream when agent changes.
  useEffect(() => {
    if (!agent) {
      setAllActivities([]);
      setIsConnected(false);
      seenIdsRef.current.clear();
      return;
    }

    let cancelled = false;

    const connect = async () => {
      // Load initial history.
      try {
        const history = await getActivity(agent.id, agent.baseUrl);
        if (!cancelled) {
          seenIdsRef.current = new Set(history.map((a) => a.id));
          setAllActivities(history);
        }
      } catch {
        // History load failed — not critical, SSE will catch up.
      }

      if (cancelled) return;

      // Open SSE stream.
      const es = createActivityStream(agent.id, agent.baseUrl);
      eventSourceRef.current = es;

      es.onopen = () => {
        if (!cancelled) setIsConnected(true);
      };

      es.onmessage = (event) => {
        if (cancelled) return;
        try {
          const record: ActivityRecord = JSON.parse(event.data);
          if (seenIdsRef.current.has(record.id)) return;
          seenIdsRef.current.add(record.id);
          setAllActivities((prev) => [...prev, record]);
        } catch {
          // Ignore parse errors.
        }
      };

      es.onerror = () => {
        if (!cancelled) setIsConnected(false);
      };
    };

    connect();

    return () => {
      cancelled = true;
      if (eventSourceRef.current) {
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
    };
  }, [agent]);

  // Filter activities by session key when one is active.
  const activities = useMemo(() => {
    if (!sessionKey) return allActivities;
    return allActivities.filter((a) => a.session_id === sessionKey);
  }, [allActivities, sessionKey]);

  const send = useCallback(
    async (message: string, mode: AgentMode = "ask", files?: FileAttachment[]) => {
      if (!agent || !message.trim()) return;
      setIsSending(true);
      setError(null);

      try {
        await sendMessage(agent.id, agent.baseUrl, message, mode, files, sessionKey ?? undefined);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Send failed");
      } finally {
        setIsSending(false);
      }
    },
    [agent, sessionKey],
  );

  const clearError = useCallback(() => setError(null), []);

  return { activities, isSending, isConnected, error, clearError, send };
}
