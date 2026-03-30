import { useQuery } from "@tanstack/react-query";
import { get, agentKey, DISABLED_KEY, BASE } from "./http";
import type { ActivityRecord } from "@/lib/types";

export function useActivity(agentId: string | undefined, baseUrl: string | undefined) {
  return useQuery<ActivityRecord[]>({
    queryKey: agentId ? agentKey(agentId, "activity") : DISABLED_KEY,
    queryFn: () => get(`/api/v1/agents/${agentId}/activity?${BASE(baseUrl!)}&limit=128`),
    enabled: !!agentId && !!baseUrl,
    staleTime: Infinity,
  });
}

export function useActivityStream(agentId: string | undefined, baseUrl: string | undefined) {
  return useQuery<EventSource | null>({
    queryKey: agentId ? agentKey(agentId, "stream") : DISABLED_KEY,
    queryFn: () =>
      agentId && baseUrl
        ? new EventSource(`/api/v1/agents/${agentId}/activity/stream?${BASE(baseUrl)}`)
        : null,
    enabled: !!agentId && !!baseUrl,
    staleTime: Infinity,
    gcTime: 0,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
    retry: false,
  });
}
