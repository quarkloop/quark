"use client";

import { useState, useCallback, useEffect, useRef } from "react";
import type { ActivityRecord, AgentConnection, AgentMode, FileAttachment } from "@/lib/types";
import {
  sendMessage,
  getActivity,
  createActivityStream,
} from "@/lib/api-client";

export function useChat(agent: AgentConnection | undefined) {
  const [activities, setActivities] = useState<ActivityRecord[]>([]);
  const [isSending, setIsSending] = useState(false);
  const [isConnected, setIsConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const seenIdsRef = useRef(new Set<string>());

  // Load history and start SSE stream when agent changes.
  useEffect(() => {
    if (!agent) {
      setActivities([]);
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
          setActivities(history);
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
          setActivities((prev) => [...prev, record]);
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

  const send = useCallback(
    async (message: string, mode: AgentMode = "ask", files?: FileAttachment[]) => {
      if (!agent || !message.trim()) return;
      setIsSending(true);
      setError(null);

      try {
        await sendMessage(agent.id, agent.baseUrl, message, mode, files);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Send failed");
      } finally {
        setIsSending(false);
      }
    },
    [agent],
  );

  return { activities, isSending, isConnected, error, send };
}
