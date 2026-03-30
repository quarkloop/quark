import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get, post, agentKey, DISABLED_KEY, BASE } from "./http";
import type { Plan } from "@/lib/types";

export function usePlan(agentId: string | undefined, baseUrl: string | undefined) {
  return useQuery<Plan | null>({
    queryKey: agentId ? agentKey(agentId, "plan") : DISABLED_KEY,
    queryFn: async () => {
      try {
        return await get(`/api/v1/agents/${agentId}/plan?${BASE(baseUrl!)}`);
      } catch {
        return null;
      }
    },
    enabled: !!agentId && !!baseUrl,
    refetchInterval: 5000,
  });
}

export function useApprovePlan(agentId: string, baseUrl: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => post<Plan>(`/api/v1/agents/${agentId}/plan/approve?${BASE(baseUrl)}`),
    onSuccess(plan) {
      qc.setQueryData<Plan | null>(agentKey(agentId, "plan"), plan);
    },
  });
}

export function useRejectPlan(agentId: string, baseUrl: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => post<void>(`/api/v1/agents/${agentId}/plan/reject?${BASE(baseUrl)}`),
    onSuccess() {
      qc.setQueryData<Plan | null>(agentKey(agentId, "plan"), null);
    },
  });
}
