import { useMutation } from "@tanstack/react-query";
import { post, BASE } from "./http";
import type { AgentMode, FileAttachment, ChatResponse } from "@/lib/types";
import { CHAT_TIMEOUT_MS } from "@/lib/constants";

export function useSendMessage(agentId: string | undefined, baseUrl: string | undefined) {
  return useMutation({
    mutationFn: async (vars: {
      message: string;
      mode?: AgentMode;
      files?: FileAttachment[];
      sessionKey?: string;
    }): Promise<ChatResponse> => {
      const url = `/api/v1/agents/${agentId}/chat?${BASE(baseUrl!)}`;
      const { message, mode = "ask", files, sessionKey } = vars;

      if (files && files.length > 0) {
        const form = new FormData();
        form.append("message", message);
        form.append("mode", mode);
        if (sessionKey) form.append("session_key", sessionKey);
        for (const f of files) {
          form.append("files", f.file, f.name);
        }
        const res = await fetch(url, {
          method: "POST",
          body: form,
          signal: AbortSignal.timeout(CHAT_TIMEOUT_MS),
        });
        if (!res.ok) {
          const err = await res.json().catch(() => ({ error: res.statusText }));
          throw new Error(err.error || res.statusText);
        }
        return res.json();
      }

      return post<ChatResponse>(url, { message, mode, session_key: sessionKey });
    },
    retry: false,
  });
}
