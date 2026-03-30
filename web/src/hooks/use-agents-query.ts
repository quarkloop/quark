import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get, AGENTS_KEY } from "./http";
import type { AgentConnection } from "@/lib/types";

export function useAgents() {
  return useQuery<AgentConnection[]>({
    queryKey: AGENTS_KEY,
    queryFn: () => get("/api/v1/agents/discover"),
    refetchInterval: 30_000,
  });
}

export function useAddAgent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (agent: AgentConnection) => agent,
    onSuccess(agent) {
      qc.setQueryData<AgentConnection[]>(AGENTS_KEY, (prev) => {
        if (!prev) return [agent];
        if (prev.some((a) => a.id === agent.id)) return prev;
        return [...prev, agent];
      });
    },
  });
}

export function useRemoveAgent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => id,
    onSuccess(id) {
      qc.setQueryData<AgentConnection[]>(AGENTS_KEY, (prev) =>
        prev ? prev.filter((a) => a.id !== id) : [],
      );
    },
  });
}
