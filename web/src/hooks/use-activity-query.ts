import { useQuery } from "@tanstack/react-query";
import { get, agentKey, DISABLED_KEY, BASE } from "./http";
import type { ActivityRecord } from "@/lib/types";

export function activityKey(agentId: string, sessionKey?: string | null) {
  return agentKey(agentId, sessionKey ? `activity:${sessionKey}` : "activity");
}

export function useActivity(
  agentId: string | undefined,
  baseUrl: string | undefined,
  sessionKey?: string | null,
  spaceId?: string,
) {
  return useQuery<ActivityRecord[]>({
    queryKey: agentId ? activityKey(agentId, sessionKey) : DISABLED_KEY,
    queryFn: () =>
      sessionKey
        ? get(
            `/api/v1/agents/${agentId}/sessions/${encodeURIComponent(sessionKey)}/activity?${BASE(baseUrl!, spaceId)}&limit=128`,
          )
        : get(`/api/v1/agents/${agentId}/activity?${BASE(baseUrl!, spaceId)}&limit=128`),
    enabled: !!agentId && !!baseUrl && (!!sessionKey || !!spaceId),
    staleTime: Infinity,
  });
}

export function useActivityStream(
  agentId: string | undefined,
  baseUrl: string | undefined,
  sessionKey?: string | null,
  spaceId?: string,
) {
  return useQuery<EventSource | null>({
    queryKey: agentId ? agentKey(agentId, `stream:${sessionKey ?? "all"}`) : DISABLED_KEY,
    queryFn: () =>
      agentId && baseUrl && sessionKey
        ? new EventSource(
            `/api/v1/agents/${agentId}/sessions/${encodeURIComponent(sessionKey)}/activity/stream?${BASE(baseUrl, spaceId)}`,
          )
        : null,
    enabled: !!agentId && !!baseUrl && !!sessionKey,
    staleTime: Infinity,
    gcTime: 0,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    retry: false,
  });
}
