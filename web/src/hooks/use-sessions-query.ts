import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get, post, del, agentKey, DISABLED_KEY, BASE } from "./http";
import type { SessionRecord, CreateSessionRequest, CreateSessionResponse } from "@/lib/types";

function sessionsKey(agentId: string) {
  return agentKey(agentId, "sessions");
}

export function useSessions(
  agentId: string | undefined,
  baseUrl: string | undefined,
  spaceId?: string,
) {
  return useQuery<SessionRecord[]>({
    queryKey: agentId ? sessionsKey(agentId) : DISABLED_KEY,
    queryFn: () =>
      get(`/api/v1/agents/${agentId}/sessions?${BASE(baseUrl!, spaceId)}`),
    enabled: !!agentId && !!baseUrl,
  });
}

export function useCreateSession(agentId: string, baseUrl: string, spaceId?: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: CreateSessionRequest) =>
      post<CreateSessionResponse>(
        `/api/v1/agents/${agentId}/sessions?${BASE(baseUrl, spaceId)}`,
        req,
      ),
    onSuccess(resp) {
      if (!resp.session) return;
      qc.setQueryData<SessionRecord[]>(sessionsKey(agentId), (prev) =>
        prev ? [...prev, resp.session!] : [resp.session!],
      );
    },
  });
}

export function useDeleteSession(agentId: string, baseUrl: string, spaceId?: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (sessionKey: string) =>
      del(
        `/api/v1/agents/${agentId}/sessions/${encodeURIComponent(sessionKey)}?${BASE(baseUrl, spaceId)}`,
      ),
    onSuccess(_, sessionKey) {
      qc.setQueryData<SessionRecord[]>(sessionsKey(agentId), (prev) =>
        prev ? prev.filter((s) => s.key !== sessionKey) : [],
      );
    },
  });
}
