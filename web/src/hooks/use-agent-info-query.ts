import { useQuery } from "@tanstack/react-query";
import { get, agentKey, DISABLED_KEY, BASE } from "./http";
import type { HealthResponse, ModeResponse, StatsResponse } from "@/lib/types";

export function useHealth(agentId: string | undefined, baseUrl: string | undefined) {
  return useQuery<HealthResponse>({
    queryKey: agentId ? agentKey(agentId, "health") : DISABLED_KEY,
    queryFn: () => get(`/api/v1/agents/${agentId}/health?${BASE(baseUrl!)}`),
    enabled: !!agentId && !!baseUrl,
    refetchInterval: 10_000,
  });
}

export function useMode(agentId: string | undefined, baseUrl: string | undefined) {
  return useQuery<ModeResponse>({
    queryKey: agentId ? agentKey(agentId, "mode") : DISABLED_KEY,
    queryFn: () => get(`/api/v1/agents/${agentId}/mode?${BASE(baseUrl!)}`),
    enabled: !!agentId && !!baseUrl,
  });
}

export function useStats(agentId: string | undefined, baseUrl: string | undefined) {
  return useQuery<StatsResponse>({
    queryKey: agentId ? agentKey(agentId, "stats") : DISABLED_KEY,
    queryFn: () => get(`/api/v1/agents/${agentId}/stats?${BASE(baseUrl!)}`),
    enabled: !!agentId && !!baseUrl,
  });
}
